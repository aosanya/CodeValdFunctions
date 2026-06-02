# FEAT-20260602-001 — `start-pipeline` function

**Status:** 📋 Not Started
**Severity:** High — entry point for every orchestrated pipeline; without it no `WorkflowRun` is created and the `/workflow-runs` UI stays empty regardless of what tasks/todos/jobs run
**Owner:** CodeValdFunctions
**Estimated effort:** ~1–1.5 days (new function binary + manifest + plan registration + integration test)
**Source finding:** This conversation (2026-06-02) — surfaced while researching why scenario [`/4-QA/agencies/utility-app-builder/09`](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/09/) almost always fails — there is no actor on the path that creates a `WorkflowRun`, so the closure has no anchor and the UI shows no rows

---

## Problem

The `next-task-selector` plan ([scenario-09/00-setup Step 10c](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/09/00-setup.md)) is the current canonical entrypoint for the Part-G pipeline: a publish to `work.next.requested` causes CodeValdFunctions to run `next-task`, which picks the lowest incomplete task and assigns it to a developer-role agent. The `work.task.assigned` event then fans out through AI decomposition → todos → compile → merge.

**Nothing on that path creates a `WorkflowRun`.** As a result:

- The `/agencies/utility-app-builder/workflow-runs` UI is empty after a full pipeline run.
- There is no transaction handle to roll back a failed run.
- Operators have no way to look at "everything from this one pipeline" — only "this one task" or "this one job."

The user's [stated requirement](memory:user) (this conversation): *"everything pertaining a task must be under a task run. WE must have entries in `/workflow-runs` when it runs."*

## Goal

Introduce a new function plan that fires **before** `next-task-selector`. Its single responsibility is to mint a `WorkflowRun` and emit the lifecycle envelope events.

- Function code: `start-pipeline`
- Handler service: `codevaldfunctions`
- Trigger topic: `work.pipeline.requested`
- Effect:
  1. Calls `POST /work/{agencyId}/workflow-runs` (or the gRPC equivalent) to create a `WorkflowRun` with the inbound `name` / `trigger_event` / `initiator`. If `name` is empty, generates one (e.g. `pipeline-YYYY-MM-DD-HHMMSS-<6hex>`).
  2. Publishes `work.pipeline.started` with `{ workflow_run_id, name, trigger_event, initiator, started_at }` — confirmation event, consumed by the UI's `LiveProgressBanner` and by the QA scenario to learn the run-id.
  3. Publishes `work.next.requested` with `{ workflow_run_id }` — chains into the existing `next-task-selector` plan, which now sees the run-id in its inbound payload and propagates it onto every artifact downstream (see [FEAT-20260602-001 umbrella](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md), §4 chain-through rule).

## Non-goals

- The `WorkflowRun.name` schema change itself — owned by [FEAT-20260602-001 in CodeValdWork](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_name.md).
- `WorkflowRun.status` transitions — owned by [FEAT-20260602-003 in CodeValdWork](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-003_workflow_run_status_state_machine.md). `start-pipeline` only writes the initial `pending` row.
- Wide propagation of `workflow_run_id` onto domain entities — owned by the per-service sibling FEATs.

---

## Design

### Trigger payload (`work.pipeline.requested`)

```json
{
  "agency_id":     "utility-app-builder",
  "name":          "qa-scenario-09-2026-06-02-150412",   // optional
  "trigger_event": "qa.scenario-09",                       // free-form label, shown in UI
  "initiator":     "qa-runner"                             // who started this
}
```

All three of `name`, `trigger_event`, `initiator` are optional. If `name` is empty, server-generated. If `trigger_event` is empty, defaults to `work.pipeline.requested`. If `initiator` is empty, set to the calling service name (resolved by Cross from the publisher identity, or `"unknown"` if not resolvable).

### Function body (pseudocode)

```go
func StartPipeline(ctx context.Context, evt PipelineRequestEvent) error {
    name := evt.Name
    if name == "" {
        name = generateName(time.Now())   // e.g. "pipeline-2026-06-02-150412-a3f1"
    }
    run, err := workClient.CreateWorkflowRun(ctx, CreateWorkflowRunRequest{
        AgencyID:     evt.AgencyID,
        Name:         name,
        TriggerEvent: orDefault(evt.TriggerEvent, "work.pipeline.requested"),
        Initiator:    orDefault(evt.Initiator,    "unknown"),
    })
    if err != nil {
        return fmt.Errorf("CreateWorkflowRun: %w", err)
    }
    // Confirmation event — UI/test subscribers correlate by name
    if err := bus.Publish(ctx, "work.pipeline.started", PipelineStartedEvent{
        WorkflowRunID: run.ID,
        Name:          run.Name,
        TriggerEvent:  run.TriggerEvent,
        Initiator:     run.Initiator,
        StartedAt:     run.StartedAt,
    }); err != nil {
        return fmt.Errorf("publish work.pipeline.started: %w", err)
    }
    // Chain into existing next-task-selector plan
    return bus.Publish(ctx, "work.next.requested", NextRequestedEvent{
        AgencyID:      evt.AgencyID,
        WorkflowRunID: run.ID,
    })
}
```

### Plan registration (idempotent — same shape as Steps 10c/11/11b/13 in 00-setup.md)

```bash
curl -s -X POST "${BASE}/agency/utility-app-builder/work-plans" \
  -u "$CV_AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "code":            "start-pipeline",
    "trigger_topic":   "work.pipeline.requested",
    "function_code":   "start-pipeline",
    "handler_service": "codevaldfunctions",
    "enabled":         true
  }'
```

### Manifest (`functions/start-pipeline.json`)

```json
{
  "code": "start-pipeline",
  "binary": "start-pipeline",
  "trigger_topic": "work.pipeline.requested",
  "publishes": ["work.pipeline.started", "work.next.requested"]
}
```

### Registrar update

Add `work.pipeline.requested` to the Consumes list in `internal/registrar/registrar.go`. After deploy, restart CodeValdFunctions so it re-registers with the new topic.

---

## Implementation plan

### Phase 1 — Function binary

1. New `functions/start-pipeline/main.go` (or equivalent) implementing the function body above.
2. New manifest `functions/start-pipeline.json`.
3. Add `work.pipeline.requested` to `internal/registrar/registrar.go` Consumes.
4. `make build` + `docker compose build codevaldfunctions` + restart.

### Phase 2 — Wiring on the QA path

1. Add Step 0 to [09-work-01-task-create-assign.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/09/09-work-01-task-create-assign.md) (see [scenario-09 docs update FEAT](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-002_scenario_09_workflow_run_step0.md)).
2. Add an idempotent setup step to [00-setup.md](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/09/00-setup.md) that ensures the `start-pipeline` plan exists, mirroring the pattern for `next-task-selector`.

### Phase 3 — Tests

- Unit: function called with empty `name` → run created with generated name; with caller-supplied `name` → that name persisted.
- Integration: publish `work.pipeline.requested` → assert one new row in `work_workflow_runs` and `work.pipeline.started` + `work.next.requested` events on the bus.

---

## Verification

- Run scenario 09 with the new Step 0 in place. Assert: `curl ${BASE}/work/utility-app-builder/workflow-runs?name=qa-scenario-09-...` returns exactly one row, status `pending`.
- Open `http://localhost:5053/agencies/utility-app-builder/workflow-runs` — row visible with the name from the test.
- `work.pipeline.started` appears in `/logs/.../stream` within 1 s of the publish.
- `next-task-selector` fires next, with `workflow_run_id` in its inbound payload (verifiable in the SSE log).

---

## Open design questions

1. **Name uniqueness.** Should `name` be unique per agency (reject duplicates with 409)? Lets the test rely on `GET ?name=...` returning a single row. Recommend yes for v1; if the caller wants a re-run, append a discriminator.
2. **Trigger publisher identity.** The QA scenario publishes `work.pipeline.requested` via `POST ${BASE}/pubsub/{agency}/publish`. Cross resolves the calling user as `initiator` if not set; without an authenticated session this is empty. Document that the test should set `initiator` explicitly so the UI row is readable.

---

## Dependencies

- Blocks: [umbrella FEAT-20260602-001](../../../../CodeValdCross/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_id_propagation_umbrella.md) — without `start-pipeline` minting runs, downstream services have no inbound `workflow_run_id` to propagate.
- Builds on: [FEAT-20260601-001 (WorkflowRun entity + `CreateWorkflowRun` RPC)](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260601-001_workflow_run_rollup.md).
- Pairs with: [FEAT-20260602-001 in CodeValdWork (`WorkflowRun.name` field)](../../../../CodeValdWork/documentation/3-SofwareDevelopment/mvp-details/FEAT-20260602-001_workflow_run_name.md).
