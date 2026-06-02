# CodeValdFunctions — MVP Tasks

Tasks moved to the platform-wide prioritization doc:

[CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md](../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md)

See the **CodeValdFunctions — MVP Task List** section at the top of that file for
MVP-FN-001 through MVP-FN-007.

---

## Outstanding feature work

| Task ID | Title | Status | Depends On |
|---|---|---|---|
| FEAT-20260602-001 | `start-pipeline` function — fires on `work.pipeline.requested`, mints a `WorkflowRun`, publishes `work.pipeline.started` (confirmation) + `work.next.requested` (handoff to `next-task-selector`); the entry point for every orchestrated pipeline | ✅ Done | FEAT-20260602-001 in CodeValdWork (`WorkflowRun.name`) |
| FEAT-20260602-002 | `workflow_run_id` on `Job` + every `functions.job.*` event payload (Functions sibling of the [Cross umbrella](../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md)) | ✅ Done | ~~FEAT-20260602-001~~ ✅ (start-pipeline) |
| FEAT-20260602-004-FUNCTIONS | CodeValdFunctions leg of WorkflowRun rollback — `FunctionsService.RollbackByWorkflowRun` cancels in-flight Jobs and freezes terminal Jobs as `rolled_back` audit; mirrors AI leg | ✅ Done | ~~FEAT-20260602-002~~ ✅ |
| DOC (Functions) | Function-job failure events, FJ-1..FJ-7 modes, `compile-solving-problem` & `merge-solving-problem` sketches, replaces `inject-compile-fix-todo` design | 🚀 In Progress | — |

See [mvp-details/FEAT-20260602-001_start_pipeline_function.md](mvp-details/FEAT-20260602-001_start_pipeline_function.md) and [mvp-details/FEAT-20260602-002_workflow_run_id_in_functions.md](mvp-details/FEAT-20260602-002_workflow_run_id_in_functions.md).
