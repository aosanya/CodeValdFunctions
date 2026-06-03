# BUG-20260603-001 (Functions) — merge-flutter-branch fires once per completed todo instead of once per task

**Status:** 📋 Open
**Severity:** High — each completed todo triggers a separate merge, bloating the git log with redundant merges and risking race conditions on the target branch
**Owner:** CodeValdFunctions (merge-flutter-branch function / dispatch manifest)
**Estimated effort:** S — add a guard in the merge-flutter-branch trigger condition or in the function body itself
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

```
Function jobs for run c4821356 (2026-06-03):
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:15:45Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:15:55Z  (×2)
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:16:06Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:16:12Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:16:14Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:16:20Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:26:06Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:26:16Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:26:18Z  (×2)
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:26:23Z  (×2)
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:26:29Z
  fn=merge-flutter-branch  status=completed  created=2026-06-03T09:26:37Z
Total: 15 merge jobs for 1 task/branch
```

Same pattern observed in the previous QA run (2026-06-03 morning) with 8 todos → 8 compile jobs → 8 merge jobs.

## Root cause

The `functions.job.completed` topic carries a `payload` that includes `function_name` and compile-result metadata. The `merge-on-compile-success` dispatch manifest's `payload_match` pattern either:

1. **Does not filter on `function_name=compile-flutter`** — any `functions.job.completed` event triggers merge, or
2. **Filters correctly on function_name** but does not also guard that the compile `status=ok` for the FINAL (last) todo — triggering on every successful compile regardless of how many remain.

The correct intent is: merge once, when ALL todos for the task are complete and the final compile succeeds. The cleanest enforcement point is the merge-flutter-branch function body: before issuing the merge, check via the Work API that all todos for the task are in a terminal state (completed/failed). If any are still pending or in-progress, return `{"status":"ok","skipped":"todos not yet complete"}` without merging.

## Fix plan

**Option A — Guard in function body (preferred):**

In `merge-flutter-branch` (Python), before the `git_merge` call:
1. Fetch all todos for the task (`GET /work/{agency}/tasks/{task_id}/todos`).
2. If any todo is non-terminal (not `TASK_TODO_STATUS_COMPLETED`, `TASK_TODO_STATUS_FAILED`, `TASK_TODO_STATUS_CANCELLED`), return early with a "not all todos complete" skip result.
3. If all todos are terminal, check whether the branch was already merged (`GET /git/{agency}/branches/{branch}` — if the branch head equals main's head, skip).
4. Only then call the merge.

This is idempotent: the last completed todo's compile job will be the one that actually merges; earlier ones exit early.

**Option B — Payload match guard in dispatch manifest:**

Extend the `payload_match` condition on the `merge-on-compile-success` plan to include a field that signals "this is the last compile for the task." This would require `compile-flutter` to look up remaining todos and embed `is_last=true` in its result payload. More coupling but avoids the Work API call in the merge function.

Option A is recommended because it keeps the guard co-located with the merge logic and does not require changes to `compile-flutter`.

## Verification

1. Run a task with N > 1 todos.
2. Observe: N compile-flutter jobs fire.
3. Observe: exactly 1 merge-flutter-branch job merges the branch; all others return early with "skipped" or are not triggered.
4. Confirm: git log on main shows exactly 1 merge commit for the task.

## Dependencies

None for Option A. Option B depends on changes to `compile-flutter` output schema.
