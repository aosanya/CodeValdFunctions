# BUG-20260603-001 (Functions) — merge-flutter-branch fires once per completed todo instead of once per task

**Status:** ✅ Fixed 2026-06-03 — branch HEAD idempotency check added; compares feature branch `headCommitId` against main `headCommitId` before calling merge API; skips if already equal (fast-forward merge already applied)
**Severity:** High — each completed todo triggers a separate merge, bloating the git log with redundant merges and risking race conditions on the target branch
**Owner:** CodeValdFunctions (merge-flutter-branch function / dispatch manifest)
**Estimated effort:** M — fix requires changing either the pipeline trigger or adding branch-idempotency check in merge-flutter-branch
**Source finding:** QA scenario 09 run 2026-06-03 — 13 completed todos → 15 compile-flutter jobs → 15 merge-flutter-branch jobs for a single task/branch; expected: 1 merge job total

## Problem

The pipeline is designed as:
```
work.todo.completed → compile-on-todo-completed → compile-flutter (per todo)
functions.job.completed (compile-flutter) → merge-on-compile-success → merge-flutter-branch (per compile)
```

With N completed todos, N compile-flutter jobs fire, and each successful compile triggers one merge-flutter-branch. The result is N merge jobs for a single task, all merging the same branch. In this QA run with 13 completed todos: 15 compile-flutter jobs, 15 merge-flutter-branch jobs. Each merge-flutter-branch call merges `feature/SF-001_scaffolding` to main, producing a redundant "already up to date" or a true merge commit depending on timing.

This creates:
- Noise in the git log (15 merge commits / no-ops for one logical task)
- Race conditions if two merge jobs run concurrently (second job may fail with a conflict or succeed on a stale HEAD)
- Misleading WorkflowRun Functions Jobs tab (15 entries instead of 1)

## Evidence

**QA run 2026-06-03T09:05 UTC (run c4821356):**
```
Total: 15 merge jobs (15 compile-flutter jobs) for 1 task/branch
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:15:45Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:15:55Z  (×2)
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:16:06Z
  ... (15 total)
```

**QA run 2026-06-03T19:33 UTC (run 69da2d61) — guard present but ineffective:**
```
Compile jobs (all status=completed result.status=ok):
  fn=compile-flutter  created=2026-06-03T19:34:29Z  branch=feature/MVP-SF-012-implement-app-version-display-widget
  fn=compile-flutter  created=2026-06-03T19:34:39Z  branch=feature/MVP-SF-012-...
  fn=compile-flutter  created=2026-06-03T19:34:51Z  branch=feature/MVP-SF-012-...
  fn=compile-flutter  created=2026-06-03T19:35:03Z  branch=feature/mvp-sf-012-...
  fn=compile-flutter  created=2026-06-03T19:35:11Z  branch=feature/mvp-sf-012-...
  fn=compile-flutter  created=2026-06-03T19:35:18Z  branch=feature/mvp-sf-012-...
Merge jobs (all status=completed result=merged):
  fn=merge-flutter-branch  created=2026-06-03T19:34:33Z  merged=feature/MVP-SF-012-...
  fn=merge-flutter-branch  created=2026-06-03T19:34:41Z  merged=feature/MVP-SF-012-...
  fn=merge-flutter-branch  created=2026-06-03T19:34:54Z  merged=feature/MVP-SF-012-...
  fn=merge-flutter-branch  created=2026-06-03T19:35:06Z  merged=feature/mvp-sf-012-...
  fn=merge-flutter-branch  created=2026-06-03T19:35:13Z  merged=feature/mvp-sf-012-...
  fn=merge-flutter-branch  created=2026-06-03T19:35:20Z  merged=feature/mvp-sf-012-...
Total: 6 compile + 6 merge for 1 task (MVP-SF-012, 3 todos)
```

The guard code was added after the first run, but the second run confirms it is not effective.

## Root cause

**Original root cause (pre-guard):** `compile-on-todo-completed` fires once per `work.todo.completed`, producing N compile jobs for N todos. Each successful compile triggers `merge-on-compile-success`, producing N merge jobs.

**Updated root cause (guard present but ineffective — race condition):**

Code inspection of `CodeValdFunctions/manager.go publish()` confirms that `workflow_run_id` IS included in `functions.job.completed` event payloads (via `job.WorkflowRunID`). The guard's field-presence assumption was incorrect — `event.get("workflow_run_id", "")` does return a value.

The real failure is a **timing race**. The guard logic is:

```python
# query todos for workflow_run_id; skip merge if any non-terminal
TERMINAL = {"TODO_STATUS_COMPLETED", "TODO_STATUS_FAILED", "TODO_STATUS_CANCELLED"}
non_terminal = [t for t in todos if t.get("status", "") not in TERMINAL]
if non_terminal:
    return  # skip
# else: proceed with merge
```

Each compile-flutter job takes ~10 seconds. All todos complete within the first few seconds of the pipeline (the AI dispatches them quickly). By the time any compile job finishes and merge-flutter-branch runs, **all todos are already in terminal state**. The guard therefore always finds `non_terminal = []` and allows every merge to proceed.

Confirmed via QA run 69da2d61: querying `task-todos?workflow_run_id=69da2d61-...` returned 6 todos, all in `TODO_STATUS_COMPLETED` or `TODO_STATUS_FAILED` — none non-terminal. Every merge guard passes, every merge executes.

## Fix plan

The todos-terminal guard cannot prevent the race condition because terminal-status is always reached before merge time. A different dedup signal is needed.

**Option A — Branch-already-merged idempotency check (recommended, minimal change):**

Before calling the merge API, check whether the feature branch is already merged into main. If `git log main..feature/BRANCH` is empty (branch HEAD is already reachable from main), exit early with `skipped: already merged`.

```python
# After resolving branch_name from inp/event:
import subprocess, json

# Check if branch is already merged into main
result = subprocess.run(
    ["git", "log", f"main..{branch_name}", "--oneline"],
    capture_output=True, text=True, cwd=repo_path
)
if result.returncode == 0 and result.stdout.strip() == "":
    out("ok", result=json.dumps({"skipped": "branch already merged into main", "branch": branch_name}))
    return
```

This is idempotent regardless of how many merge jobs run concurrently — the first job succeeds, all subsequent jobs see the branch already merged and skip. No cross-service coordination needed.

**Option B — Change pipeline trigger to fire on task completion (architecturally correct):**

Change `compile-on-todo-completed` to `compile-on-task-completed` — trigger compile only when `work.task.completed` fires (once per task) instead of `work.todo.completed` (once per todo). This eliminates N compile jobs at the source and makes the merge-guard problem moot.

This requires:
1. A new plan trigger `compile-on-task-completed` that subscribes to `work.task.completed`
2. Removing or disabling `compile-on-todo-completed`
3. Passing `task_id` and `branch_name` from the task-completed payload to compile-flutter

Option B is architecturally correct but changes the pipeline topology. Option A is a targeted fix that does not change the trigger fan-out behavior.

**Option A is recommended** as the most reliable short-term fix. Option B should be considered as a follow-up cleanup.

## Verification

1. Run a task with N > 1 todos.
2. Observe: N compile-flutter jobs fire (expected; trigger not changed under Option A).
3. Observe: exactly 1 merge-flutter-branch job merges the branch; all others return early with `skipped: already merged`.
4. Confirm: git log on main shows exactly 1 merge commit for the task.

## Dependencies

None for Option A. Option B depends on `work.task.completed` event payload carrying `branch_name` or the branch being derivable from `task_id`.
