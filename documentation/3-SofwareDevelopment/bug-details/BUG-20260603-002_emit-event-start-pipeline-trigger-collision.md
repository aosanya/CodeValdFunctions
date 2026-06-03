# BUG-20260603-002 — `emit-event` and `start-pipeline` share trigger `work.pipeline.requested` with no payload guard

**Status:** ✅ Fixed (2026-06-03)
**Severity:** High — non-deterministic pipeline dispatch. `Lookup` returns the alphabetically-first manifest, so the FEAT-20260602-005 helper `emit-event` wins by default and the actual orchestrator `start-pipeline` is silently skipped. Causes pipeline-requested events to no-op when they should mint a WorkflowRun.
**Owner:** CodeValdFunctions
**Estimated effort:** ~0.1 day (single-line manifest change)
**Source finding:** QA scenario 11 retry pass (2026-06-03T21:46 UTC) — operator published `work.pipeline.requested` to validate the unblocked MVP-SF-002 chain; `codevaldfunctions` ACKed and created a job for `emit-event` instead of `start-pipeline`, so no WorkflowRun was created and the chain never advanced.

## Problem

Both `functions/start-pipeline.json` and `functions/emit-event.json` registered the same trigger:

```json
// start-pipeline.json
{ "name": "start-pipeline", "trigger": "work.pipeline.requested", ... }

// emit-event.json (pre-fix)
{ "name": "emit-event",     "trigger": "work.pipeline.requested", ... }
```

`BinaryRunner.Lookup` (`internal/functions/runner.go`) iterates the directory and returns the first manifest whose trigger matches and whose `payload_match` qualifiers pass. With no qualifiers on either manifest, both match every `work.pipeline.requested` event. `os.ReadDir` returns entries alphabetically on Linux, so `emit-event` wins.

Historical evidence: `functions.job.created` events show 7 start-pipeline + 1 emit-event runs over the day — start-pipeline usually won by accident (race / cache). This retry it didn't.

## Evidence

```
codevaldfunctions-1  | 2026/06/03 21:46:38 codevaldfunctions: NotifyEvent: ACK event_id=b016e6eb-... topic=work.pipeline.requested source=qa-runner
codevaldfunctions-1  | 2026/06/03 21:46:38 registrar: publish topic="functions.job.created" agencyID="utility-app-builder"
```

Most-recent functions.job.created showed `function_name: emit-event` (not start-pipeline). Workflow-runs list confirmed no new run was minted; the next call to `next-task` had nothing to pick up.

## Root cause

`emit-event` is a generic publish helper intended only for recovery pipelines that supply `function_code: "emit-event"` and a `function_params` JSON describing the event to publish (FEAT-20260602-005). Without a `payload_match` guard the helper inadvertently competes with `start-pipeline` for every `work.pipeline.requested` event.

## Fix

Add a `payload_match` qualifier to `emit-event.json` so the helper only activates when a recovery pipeline explicitly invokes it:

```json
{
  "name": "emit-event",
  "trigger": "work.pipeline.requested",
  "payload_match": { "function_code": "emit-event" },
  "description": "...payload_match guard ensures this helper only activates when explicitly invoked..."
}
```

`payloadMatches` (`runner.go`) does substring-in-JSON-string matching for `"key":"value"` pairs, so this requires the literal text `"function_code":"emit-event"` to appear in the published payload — exactly the FEAT-005 contract.

`start-pipeline` becomes the unique match for plain pipeline-requested events (the operator-initiated path). Recovery pipelines that already publish `function_code:emit-event` continue to work.

## Verification

```bash
# After rebuilding codevaldfunctions and republishing a plain pipeline event:
curl -s "${BASE}/work/utility-app-builder/workflow-runs?name=qa-scenario-11-retry2-..." \
  | jq '.runs[0].status'
# WORKFLOW_RUN_STATUS_PENDING  (start-pipeline minted the run, no longer hijacked by emit-event)

# Most recent functions.job.created shows start-pipeline, not emit-event.
```

## Dependencies

- None. Single-file manifest change. The BinaryRunner is hot-rescan so a container rebuild only matters because the manifest file lives inside the image.

## Alternative considered (rejected)

- **Lookup prefers payload-qualified entries**: changing `runner.go` to sort matches so qualified manifests win first. More robust general fix, but touches shared dispatch semantics and might mask future configuration bugs. The targeted payload_match is enough for the only known offender.
