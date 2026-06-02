# FEAT-20260602-002 — `workflow_run_id` propagation in CodeValdFunctions

**Status:** 📋 Not Started
**Severity:** High — sibling of the umbrella; Jobs (compile, merge, next-task, start-pipeline) are the orchestrator's primary output; without this their relationship to a pipeline closure is invisible
**Owner:** CodeValdFunctions
**Estimated effort:** ~1.5 days (schema + Job model + payload propagation + list filter + integration tests)
**Source finding:** This conversation (2026-06-02) — sibling of [umbrella FEAT-20260602-001 in Cross](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md)

---

## Problem

CodeValdFunctions creates one `Job` row per function invocation (compile-flutter, merge-flutter-branch, next-task, inject-compile-fix-todo, and shortly `start-pipeline`). Today these jobs carry no link to the originating `WorkflowRun`, which means:

- The closure view at `/agencies/.../workflow-runs/{id}` shows job IDs as opaque strings (per the existing `function_job_ids` array on `WorkflowRun`), but can't render rich row data (status, duration, error).
- Rollback can't list "every job from this run" without scanning the entire `function_jobs` collection.
- Diagnostic flows (e.g. compile-fix retry, merge-failure-diagnostics) start new jobs that should clearly belong to the same pipeline; today they're orphans.

## Goal

Make `workflow_run_id` a first-class typed field on:

- `Job` entity
- Every `functions.job.*` event payload (`functions.job.created`, `functions.job.completed`, `functions.job.failed`)
- `ListJobs` RPC / `GET /functions/{agencyId}/jobs?workflow_run_id=X` filter

## Non-goals

- Adding `workflow_run_id` to function manifests, function-code definitions, or other config entities.
- Changing function execution semantics or the manifest schema (only the inbound event reading + Job row + outbound event publishing).

---

## Design

### Schema change

In `schema.go`, under the `Job` `TypeDefinition`:

```go
{Name: "workflow_run_id", Type: types.PropertyTypeString},
```

### Proto change

In `proto/codevaldfunctions/v1/`:

- `Job` message: `string workflow_run_id = N;`
- `ListJobsRequest` accepts `string workflow_run_id` filter.

### Event payload changes

Every event in [`internal/server/job_lifecycle.go`](../../internal/server/job_lifecycle.go) (or equivalent) gains `workflow_run_id`. Read from the inbound trigger event when creating the Job; persist on the row; include in every emitted event.

### Chain-through behaviour

Every function plan is triggered by an event from the bus. The function dispatcher reads `workflow_run_id` from the trigger payload and stamps it on the new `Job` row. Per-function chain-through:

| Function | Triggered by | Reads `workflow_run_id` from | Writes onto |
|---|---|---|---|
| `start-pipeline` | `work.pipeline.requested` | (none — mints the WorkflowRun) | the new WorkflowRun + `work.pipeline.started` + `work.next.requested` |
| `next-task` | `work.next.requested` | inbound event | Job row + the resulting `work.task.assigned` event |
| `compile-flutter` | `work.todo.completed` | inbound event | Job row + `functions.job.completed` event |
| `inject-compile-fix-todo` | `functions.job.completed` (compile w/ issues) | inbound event | Job row + the new TaskTodo (via Work API) + emitted event |
| `merge-flutter-branch` | `functions.job.completed` (compile ok) | inbound event | Job row + the resulting `git.merge.*` event |

### List filter

`GET /functions/{agencyId}/jobs?workflow_run_id=X` returns the jobs the run produced. The closure SSE endpoint ([FEAT-20260602-003 in Cross](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-003_workflow_run_closure_sse_aggregation.md)) calls this.

### `start-pipeline` interaction

`start-pipeline` is special — it's the only function that **mints** the `workflow_run_id` (every other function inherits it from the trigger). Its own `Job` row should have `workflow_run_id` set to the newly-created run's ID after the `CreateWorkflowRun` call returns (i.e., persisted *after* the run exists). The published `work.pipeline.started` carries the same. See [FEAT-20260602-001 (start-pipeline)](FEAT-20260602-001_start_pipeline_function.md) for the function-specific design.

---

## Implementation plan

### Phase 1 — Schema + proto (~0.5 day)

1. Add property to `Job` in `schema.go`.
2. Add proto field; `make proto`.

### Phase 2 — Dispatcher + per-function handlers (~0.5 day)

1. Update job creation to read `workflow_run_id` from the trigger event.
2. Update every function's published event payload.
3. Special-case `start-pipeline` to set `workflow_run_id` post-mint.

### Phase 3 — Tests (~0.5 day)

- Unit: job created with trigger payload containing `workflow_run_id` → persists; absent → empty.
- Integration: full chain — pipeline created → compile job has run-id → merge job has run-id → all `functions.job.*` events carry run-id.

---

## Verification

- `go test -race -count=1 ./...` clean.
- Run scenario 09; `GET /functions/utility-app-builder/jobs?workflow_run_id=$RUN` returns: start-pipeline, next-task, compile-flutter, merge-flutter-branch jobs (and any inject-compile-fix-todo jobs if the diagnostic loop fired).
- Each `functions.job.completed` event in the SSE log carries `workflow_run_id`.

---

## Open design questions

1. **Retry semantics.** If a job retries (compile-fix loop or auto-retry), the retried Job row inherits the same `workflow_run_id`. Confirm — recommend yes, retries are part of the same pipeline closure.
2. **Orphan jobs.** A function triggered by a non-pipeline event (e.g. ad-hoc CLI publish) has no `workflow_run_id` in the trigger. Allow `""` for v1 per the umbrella's orphan policy.

---

## Dependencies

- Part of umbrella: [FEAT-20260602-001 in Cross](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md).
- Pairs with: [start-pipeline FEAT-20260602-001](FEAT-20260602-001_start_pipeline_function.md), [Work sibling FEAT](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-002_workflow_run_id_in_work.md), [Git sibling FEAT](../../../../CodeValdGit/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_in_git.md).
