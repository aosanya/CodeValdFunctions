---
status: 📋 Draft (2026-06-02)
owner: CodeValdFunctions
scope: function-job failure events + field contracts for recovery pipelines
source: gap analysis of `/4-QA/agencies/utility-app-builder/09`
---

# Function-Job Failure Modes

CodeValdFunctions owns **Jobs** — the unit of function invocation. Every Job
runs a pre-built binary function (`compile-flutter`, `merge-flutter-branch`,
`next-task`, `start-pipeline`, `emit-event`, `create-branch`, `delete-branch`)
and publishes `functions.job.*` events on terminal state. The 09 QA scenario
exercises `start-pipeline`, `next-task`, `compile-flutter`, and
`merge-flutter-branch` on the happy path; this doc catalogues what fails and
how recovery pipelines must respond.

This doc catalogues the failure events CodeValdFunctions emits and the field
contracts that recovery pipelines (per
[FEAT-20260602-005](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-005_failure_pipelines_synthesized_success.md))
must satisfy when they synthesize Functions success events.

The orchestration overview lives in
[CodeValdCross — pipeline-failure-handling](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/pipeline-failure-handling.md).

---

## How the dispatcher translates binary output to events

The `FunctionsDispatcher` runs the binary and reads its stdout JSON:

| Binary stdout `status` | Dispatcher action | Event published |
|---|---|---|
| `"ok"` | `CompleteJob(result)` | `functions.job.completed` |
| `"issues-found"` | `CompleteJob(result)` | `functions.job.completed` |
| `"infrastructure-error"` | `CompleteJob(result)` | `functions.job.completed` |
| `"error"` | `FailJob(error)` | `functions.job.failed` (after max retries) |
| binary panics / non-zero exit | `FailJob(err)` | `functions.job.failed` (after max retries) |

**Key gap (FJ-1):** `issues-found` and `infrastructure-error` are silently
swallowed as `job.completed`. Recovery pipelines cannot distinguish a clean
success from a compile failure by the event topic alone — they must also
inspect the Job entity's `result` field. Post FEAT-005, the event payload
must include `status` so Cross can route on it directly.

---

## Failure events CodeValdFunctions emits

| Event | When emitted | Payload fields |
|---|---|---|
| `functions.job.failed` | A Job reaches terminal `failed` after exhausting retries (max 3). Covers binary panics, non-zero exits, and explicit `"error"` status. | `job_id`, `function_name`, `trigger_event`, `workflow_run_id` |
| `functions.job.completed` (status=issues-found) | A compile job finishes with lint/analyze errors. Published as `completed` today; **must carry `status` in payload post-FEAT-005** so Cross can route it as a failure. | `job_id`, `function_name`, `status` (**new — must be added**), `trigger_event`, `workflow_run_id` |
| `functions.job.cancelled` | An in-flight Job cancelled by the rollback coordinator. Not a failure — terminal flow. | `job_id`, `function_name`, `workflow_run_id`, `previous_status`, `reason` |

> **Schema change needed:** The `publish()` helper in `manager.go` currently
> does not include the binary's `status` in the event payload. Post-FEAT-005,
> `CompleteJob` must forward the result's `status` field so `payload_condition`
> matching in Cross can distinguish `status=ok` from `status=issues-found`.

---

## Field contracts for synthesized success events

### `functions.job.completed` — compile-flutter (status=ok)

Listened for by: `merge-on-compile-success` (triggers `merge-flutter-branch`).

- **Must produce:** `job_id`, `function_name = "compile-flutter"`,
  `status = "ok"`, `workflow_run_id`
- **Job entity `result` must contain:** `{"status": "ok", "branch": "...",
  "task_name": "...", "output": "..."}` — `merge-flutter-branch` reads
  `result` via the Jobs API to get `branch` and `task_name`.
- **May differ:** `trigger_event`, `started_at`, `duration_ms` (different
  recovery binary may not time the same way)

A recovery pipeline (`compile-solving-problem`) that synthesizes this event
**must** also ensure the feature branch in the bare git repo contains the
fixed code — `merge-flutter-branch` clones the branch at merge time.

### `functions.job.completed` — merge-flutter-branch (status=ok)

Listened for by: `delete-branch-handler`, closure SSE.

- **Must produce:** `job_id`, `function_name = "merge-flutter-branch"`,
  `status = "ok"`, `workflow_run_id`
- **Job entity `result` must contain:** `{"merged": "<branch>",
  "merge_commit": "...", "merge_into": "main"}` — closure SSE reads this.
- **May differ:** `trigger_event`, `started_at`

---

## FJ-N — Function-job failure modes

### FJ-1 — compile-flutter returns `issues-found`

**Trigger:** `flutter analyze` detects lint or compile errors. The binary
writes `{"status": "issues-found", ...}` to stdout. The dispatcher calls
`CompleteJob` (not `FailJob`), so `functions.job.completed` fires — but with
a result that indicates failure.

Today: `merge-flutter-branch` reads the compile result and silently skips
the merge (`out("ok", "skipped: compile status=issues-found")`). The
`WorkflowRun` stalls — no further event fires.

**Post FEAT-005:**
1. `publish()` forwards `status` in the event payload.
2. `compile-on-todo-completed`'s `failure_event` is
   `functions.job.completed` with `payload_condition: "status=issues-found,function_name=compile-flutter"`.
3. Cross routes to `compile-solving-problem`.
4. `merge-on-compile-success` adds `payload_condition: "status=ok"` so it
   only fires on clean compiles (it already skips internally but the routing
   should enforce this too).

**Recovery:** `compile-solving-problem` — see sketch below.

---

### FJ-2 — compile-flutter binary crash / infra error

**Trigger:** `flutter analyze` or git clone fails at the OS level (OOM,
missing tool, network timeout). The binary either panics (non-zero exit) or
writes `{"status": "error", ...}` / `{"status": "infrastructure-error", ...}`.

Today: `FailJob` is called; after 3 retries `functions.job.failed` fires.
No subscriber handles it.

**Post FEAT-005:** `compile-on-todo-completed` declares
`failure_event = "functions.job.failed"`, `payload_condition =
"function_name=compile-flutter"`. Cross routes to `compile-solving-problem`.
The recovery pipeline distinguishes FJ-2 from FJ-1 by inspecting the
failed Job's `error` field — for an infra error, a simple retry (re-run the
same binary) is appropriate rather than an AI-mediated fix.

**Recovery:** `compile-solving-problem` (retry branch) → re-runs compile on
the same branch after a short delay.

---

### FJ-3 — compile-flutter detects partial branch (git-file-write race)

**Trigger:** `git.file.write` events are acknowledged before the file
appears in the bare ref store. `compile-flutter` clones the branch and finds
fewer files than expected (the todo was marked complete but not all writes
flushed). Manifests as `issues-found` with errors on files that should not
have errors, or as missing import paths.

Today: indistinguishable from FJ-1 — treated as a compile error.

**Post FEAT-005:** `compile-solving-problem` detects this case by checking
the job error contains `branch_partial` (compile-flutter writes this string
when it detects a suspicious partial-file pattern). It waits a fixed delay
(5s) and re-runs compile without any AI fix.

This is a defensive layer; the authoritative fix for BUG-09-020 remains in
CodeValdGit's storer (flush before emitting `git.file.written`).

**Recovery:** `compile-solving-problem` (retry branch, short delay).

---

### FJ-4 — merge-flutter-branch fails (conflict or 4xx)

**Trigger:** The CodeValdGit merge endpoint returns a conflict error
(`git.conflict.detected`) or a 4xx (auth, missing branch). The
`merge-flutter-branch` binary writes `{"status": "error", ...}`. After
retries, `functions.job.failed` fires.

Today: `merge-failure-diagnostics` runs but only diagnoses — it does not
recover. The `WorkflowRun` stalls.

**Post FEAT-005:**
- `merge-on-compile-success` declares `failure_event =
  "functions.job.failed"`, `payload_condition =
  "function_name=merge-flutter-branch"`, `on_failure_pipeline =
  "merge-solving-problem"`.
- `merge-solving-problem` supersedes `merge-failure-diagnostics` as a
  standalone plan — see Migration section below.

**Recovery:** `merge-solving-problem` — see sketch below.

---

### FJ-5 — start-pipeline fails

**Trigger:** `start-pipeline` cannot create a `WorkflowRun` (Work API 5xx,
malformed payload, duplicate run). The binary writes `{"status": "error"}`.

Today: `FailJob` fires; no subscriber handles `functions.job.failed` for
`function_name=start-pipeline`. The pipeline never starts.

**Post FEAT-005:** `start-pipeline`'s plan declares `on_failure_pipeline =
"default-failure-pipeline"`. On failure, Cross publishes `work.run.failed`
directly (there is no `WorkflowRun` to terminate, so the failure is logged
and the triggering work item remains `IN_PROGRESS` — the watchdog
([FEAT-006](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-006_workflow_run_watchdog.md))
covers this case).

**Recovery:** `default-failure-pipeline` → `work.run.failed`.

---

### FJ-6 — next-task finds no eligible task

**Trigger:** `next-task` queries Work for the next unassigned task but none
exist (all tasks completed, all in progress, or filtered out). The binary
writes `{"status": "ok", "result": "no-eligible-task"}` — this is NOT an
error; it's a soft outcome.

Today: `next-task` writes `ok` with a `no-eligible-task` marker. No
downstream plan triggers. The `WorkflowRun` stalls in `in_progress`.

**Post FEAT-005:** `next-task-selector` declares two outcomes:
- Success (`work.task.assigned`): forward pipeline continues.
- Soft-empty (`work.next.empty`): routed to `select-fallback-task` recovery
  pipeline, which publishes `work.run.completed` (the pipeline has no more
  work to do — this is a legitimate terminal state, not a failure).

The `next-task` binary must emit `work.next.empty` (a new event) when no
task is found, instead of emitting a silent `ok` result. This is a small
protocol change.

**Recovery:** `select-fallback-task` → `work.run.completed`.

---

### FJ-7 — Job stalls (binary acknowledged but no outcome published)

**Trigger:** A function binary is dispatched (Job in `running`) but the
process hangs, is OOM-killed, or the host crashes. No stdout is produced; no
`functions.job.*` event fires. The `WorkflowRun` stalls in `in_progress`.

Today: no detection. The Cross watchdog (FEAT-006) detects the
`WorkflowRun` as stale after `WORKFLOW_RUN_STALE_TIMEOUT` and emits
`work.run.timeout`. CodeValdWork flips tasks to `FAILED`, but individual Job
entities remain stuck in `running` indefinitely.

**Post FEAT-006:** CodeValdFunctions subscribes to `work.run.timeout` and
calls `FailJob` for every Job in `running` status for the timed-out
`workflow_run_id`. This produces `functions.job.failed`, which then routes to
the appropriate failure pipeline.

Additionally, per-Job timeouts should be added (default 10 min for compile,
5 min for merge). When elapsed, the dispatcher itself calls `FailJob` and the
failure pipeline runs without waiting for the watchdog.

**Recovery:** depends on which function stalled — routes to the same
pipeline as FJ-2 (infra error) for compile, FJ-4 for merge.

---

## Recovery pipeline sketches

### `compile-solving-problem`

Supersedes the `compile-issues-handler` + `inject-compile-fix-todo` approach.
Instead of injecting a TaskTodo into CodeValdWork and re-entering the full
AI → Compile loop, this recovery pipeline runs a targeted AI fix agent
directly and synthesizes the parent step's success event.

```json
{
  "code":                "compile-solving-problem",
  "trigger_topic":       "work.pipeline.requested",
  "payload_condition":   "\"pipeline_code\":\"compile-solving-problem\"",
  "handler_service":     "codevaldai",
  "agent_id":            "compile-fixer-agent",
  "instructions":        "The compile step for branch {failed_event.payload.branch} failed. Failure type: {failed_event.payload.status}. Compile output: {compile_job.result.output}. Edit the Dart source files on the feature branch to resolve all reported issues. Do not alter the logic — fix only the lint/type errors. When done, run compile internally and confirm status=ok before publishing success.",
  "success_event":       "functions.job.completed",
  "failure_event":       "functions.job.failed",
  "on_failure_pipeline": "compile-escalate-to-operator"
}
```

**What the recovery must produce:**

The terminal step of this pipeline must publish:
```json
{
  "topic":   "functions.job.completed",
  "payload": {
    "job_id":          "<recovery-job-id>",
    "function_name":   "compile-flutter",
    "status":          "ok",
    "workflow_run_id": "<parent-workflow-run-id>"
  }
}
```
And the recovery Job's `result` must contain:
```json
{
  "status":    "ok",
  "branch":    "<same feature branch>",
  "task_name": "<original task name>",
  "output":    ""
}
```
`merge-flutter-branch` reads `branch` and `task_name` from the compile job's
`result` — the recovery must write these fields faithfully.

**Retry vs. AI fix branching (inside the recovery pipeline):**

| `failed_event.payload.status` | Action |
|---|---|
| `issues-found` | Run AI fix agent (edit source + re-compile) |
| `error` or `infrastructure-error` | Retry compile binary after 5s delay |
| `branch_partial` (FJ-3) | Retry compile binary after 5s delay (no AI fix needed) |

This branching lives inside the recovery pipeline's AI agent instructions or
as a Function that routes on the payload; it does not require Cross to branch.

---

### `merge-solving-problem`

Supersedes `merge-failure-diagnostics` as a standalone recovery plan.
`merge-failure-diagnostics` only diagnosed and logged; this pipeline recovers
and synthesizes the merge success event.

```json
{
  "code":                "merge-solving-problem",
  "trigger_topic":       "work.pipeline.requested",
  "payload_condition":   "\"pipeline_code\":\"merge-solving-problem\"",
  "handler_service":     "codevaldai",
  "agent_id":            "merge-resolver-agent",
  "instructions":        "The merge of branch {merge_job.result.branch} into main failed. Error details: {failed_event.payload.error}. If conflict: resolve the conflicting files on the feature branch (add a resolution commit), then retry the merge. If 4xx auth: produce a Comm notification and fail terminally. If missing-branch: attempt to recreate from compile result commit. When merge succeeds, publish the success event with the merge commit details.",
  "success_event":       "functions.job.completed",
  "failure_event":       "functions.job.failed",
  "on_failure_pipeline": "default-failure-pipeline"
}
```

**What the recovery must produce:**

```json
{
  "topic":   "functions.job.completed",
  "payload": {
    "job_id":          "<recovery-job-id>",
    "function_name":   "merge-flutter-branch",
    "status":          "ok",
    "workflow_run_id": "<parent-workflow-run-id>"
  }
}
```
And the recovery Job's `result` must contain:
```json
{
  "merged":        "<feature-branch>",
  "merge_commit":  "<sha>",
  "merge_into":    "main"
}
```

**Conflict resolution approach:**

The resolver runs on the **feature branch** (adds a resolution commit), not
on a dedicated resolution branch. This matches typical PR-conflict workflows
and means the branch name in the synthesized event is unchanged.

---

## Migration: why `inject-compile-fix-todo` is replaced

The previous design (`compile-issues-handler` + `inject-compile-fix-todo`)
worked by:
1. `compile-issues-handler` plan matched on `functions.job.completed` with `function_name=compile-flutter`.
2. It called CodeValdWork to inject a new `TaskTodo` into the active task.
3. The AI picked up the injected todo and re-ran the implementation cycle.
4. Compile eventually fired again via the normal `work.todo.completed` trigger.

**Problems with this design:**
- Full re-entry into the AI implementation cycle for what is often a small
  syntax fix — expensive and slow.
- The injected todo has no `max_runs` guardrail by default; a buggy prompt
  could loop indefinitely.
- `compile-issues-handler` was never implemented (noted as NOT YET IMPLEMENTED
  in FEAT-005 doc).
- Every compile failure required a new TaskTodo entity, polluting the task's
  todo list with fix-attempts.
- Recovery was invisible — the parent `WorkflowRun` showed an extra todo but
  no indication of a failure-recovery cycle.

**Why `compile-solving-problem` is better:**
- Targeted: the AI fix agent receives exactly the compile output and edits
  only the affected files.
- Bounded: the failure pipeline's nesting cap (default 2) prevents infinite
  loops.
- Visible: the child `WorkflowRun` appears in the closure SSE as a recovery
  run — operators can see exactly what happened.
- No CodeValdWork side-effects: no spurious TaskTodo entities.
- Self-contained: the synthesized `functions.job.completed { status: ok }`
  restores the forward pipeline without anyone knowing recovery ran.

`inject-compile-fix-todo` may still have a role as the **last step** inside
`compile-solving-problem` if the AI agent decides the fix requires a full
implementation re-run (i.e. the issue is not a syntax error but a missing
feature). In that case the recovery pipeline deliberately escalates to a
full decomp cycle. This is opt-in per-agency; not the default.

---

## Open follow-ups

- **Add `status` to event payload.** `publish()` in `manager.go` must
  forward the binary result's `status` field so Cross can route on
  `issues-found` without a Jobs API call.
- **`work.next.empty` event.** `next-task` binary must emit this instead of
  silent `ok` when no eligible task is found (FJ-6).
- **Per-Job timeout.** Configurable per-function timeout (default 10 min
  compile, 5 min merge) that calls `FailJob` if the binary stalls.
- **`merge-failure-diagnostics` migration.** Embed its logic as the first AI
  step of `merge-solving-problem`; delete it as a standalone plan in
  `agency.json`.
- **compile-escalate-to-operator.** Define this terminal pipeline (emits Comm
  + `work.run.failed`) when `compile-solving-problem` exhausts its nesting cap.
- **`work.run.timeout` subscriber.** CodeValdFunctions must subscribe and call
  `FailJob` for any running Job in the timed-out `workflow_run_id` (FJ-7).

---

## Related work

- [Cross — pipeline-failure-handling](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/pipeline-failure-handling.md)
- [Cross — FEAT-20260602-005 — failure pipelines via synthesized success events](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-005_failure_pipelines_synthesized_success.md)
- [Cross — FEAT-20260602-006 — workflow-run watchdog](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-006_workflow_run_watchdog.md)
- [AI — ai-run-failure-modes](../../../../CodeValdAI/documentation/3-SofwareDevelopment/mvp-details/ai-run-failure-modes.md)
- [Git — git-failure-modes](../../../../CodeValdGit/documentation/3-SofwareDevelopment/mvp-details/git-failure-modes.md)
- [Work — task-failure-modes](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/task-failure-modes.md)
- [job-lifecycle.md](job-lifecycle.md) — Job state machine today
- [BUG-09-020 — task completes before all git.file.write events flush](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/bugs/09-mvp-sf-pipeline-findings.md)
