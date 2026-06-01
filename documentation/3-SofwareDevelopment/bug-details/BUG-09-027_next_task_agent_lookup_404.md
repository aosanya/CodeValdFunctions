# BUG-09-027 — `next-task` function 404s looking up agent by slug; PUT assignee then 404s

**Status:** ✅ Fixed (2026-06-01, working tree — not yet committed)
**Severity:** High — blocked `next-task-selector` from assigning any task even when the agent existed; entire 09 Part-G pipeline could not start
**Owner:** CodeValdWork (fixed there — `next-task` is now contract-compliant)
**Source finding:** Hit during the 09 Part-G QA run on 2026-06-01 once [BUG-09-026](../../../../CodeValdCross/documentation/3-SofwareDevelopment/bug-details/BUG-09-026_http_publish_skips_fanout.md) was worked around with grpcurl

## Resolution (2026-06-01)

Fix landed in CodeValdWork (not Functions) — `GetAgent` and `AssignTask` now match the upsert PUT's slug-first semantics:

- [`agent.go`](../../../../CodeValdWork/agent.go) — `GetAgent` now accepts either the entity UUID or the `agent_id` slug. UUID lookup is tried first; on NotFound it falls back to `GetAgentByAgentID`, which queries `ListEntities` filtered by `agent_id`. Added `GetAgentByAgentID` as a public method on `TaskManager` for direct slug lookups.
- [`assignment.go`](../../../../CodeValdWork/assignment.go) — `AssignTask` now uses the resolved `agent.ID` (UUID) for both the `assigned_to` edge's `ToID` and the `work.task.assigned` payload's `AgentID`. Previously it passed through the raw URL-param string which, with a slug, would have produced a dangling edge even if GetAgent succeeded.
- [`task.go`](../../../../CodeValdWork/task.go) — updated `TaskManager` interface doc + added `GetAgentByAgentID`.

### Verification

```
$ curl ".../work/utility-app-builder/agents/developer-01" -u "$CV_AUTH"
HTTP 200
{ "agent": { "id":"e9494417-...", "agentId":"developer-01", "roleName":"Developer", ... } }

$ curl -X POST ".../pubsub/utility-app-builder/publish" -d '{"topic":"work.next.requested",...}'
HTTP 200 {"status":"ok"}

# Poll 10s later
$ next-task-selector picked: MVP-SF-001 (1c320ad8-...)
  status:         TASK_STATUS_IN_PROGRESS
  RESUME_MODE:    false
```

Three new unit tests cover the change:
- `TestGetAgent_AcceptsSlug` — slug round-trips to the same UUID
- `TestGetAgentByAgentID_NotFound` — slug not found returns ErrAgentNotFound
- `TestAssignTask_AcceptsAgentSlug_EdgeUsesUUID` — assigning by slug writes an edge keyed on the resolved UUID

`go test -race -short ./...` passes.

---

## Problem

When `work.next.requested` is delivered with `payload={"agent_id":"developer-01"}`, the `next-task` function:

1. Picks a runnable task (works — query + filter is correct).
2. Resolves the agent: `GET /work/{agency}/agents/developer-01` → **404 agent not found**.
3. Assigns the task: `PUT /work/{agency}/tasks/{taskId}/assignee/developer-01` → **404 agent not found**.

The function then writes `functions.job.completed` with no task assignment. The pipeline halts.

## Evidence

Cross logs during a grpcurl-triggered `work.next.requested`:

```
codevaldcross-1 | proxy: GET /work/utility-app-builder/agents/developer-01 → GET /work/{agencyId}/agents/{agentId} (service=codevaldwork)
codevaldcross-1 | proxy: GET /work/utility-app-builder/agents/developer-01: Invoke /codevaldwork.v1.TaskService/GetAgent → gRPC NotFound: agent not found
codevaldcross-1 | http: GET /work/utility-app-builder/agents/developer-01 → 404 (5.04ms)
codevaldcross-1 | proxy: PUT /work/utility-app-builder/tasks/.../assignee/developer-01 → PUT /work/{agencyId}/tasks/{taskId}/assignee/{agentId} (service=codevaldwork)
codevaldcross-1 | proxy: PUT /work/utility-app-builder/tasks/.../assignee/developer-01: Invoke /codevaldwork.v1.TaskService/AssignTask → gRPC NotFound: agent not found
```

Yet earlier in the same session the setup confirmed `developer-01` exists with `roleName=Developer`:

```
$ curl -X PUT /work/utility-app-builder/agents/developer-01 ...
{ "agent": { "id":"e9494417-ecfe-4369-8bad-38da48019744",
             "agentId":"developer-01", "roleName":"Developer", ... } }
```

So the upsert PUT accepts the slug `developer-01`, but GET / AssignTask require the UUID `e9494417-...`.

## Root cause (suspected)

Two possible places, need confirmation:

1. **CodeValdWork — `GetAgent` and `AssignTask` route binding mismatch.** The PUT-upsert route at `/agents/{agentId}` uses the path param as an `agent_id` slug lookup that falls back to insert. The GET and the AssignTask routes likely bind `{agentId}` to the entity UUID field instead. If that's the case the inconsistency is the bug — either all three routes should accept the slug, or only one should.

2. **CodeValdFunctions `next-task` function logic.** If CodeValdWork's contract is "agent endpoints take UUID, not slug," then `next-task` should resolve the slug to UUID via `GET /work/{agency}/agents?agentId=developer-01` (or similar list query) before calling GetAgent / AssignTask.

The fix lands in whichever side owns the contract. Quickest verification: look at [`CodeValdWork/internal/server/`](../../../../CodeValdWork/internal/server/) for the `GetAgent` and `AssignTask` route bindings, then either align them with the upsert PUT, or have `next-task` (in [`CodeValdFunctions/functions/next-task`](../../../functions/next-task)) do the slug→UUID resolution.

## Fix plan

Phase 1 (CodeValdWork, if route mismatch):
- Make GET `/agents/{agentId}` and PUT `/tasks/{taskId}/assignee/{agentId}` accept the `agentId` slug, matching the upsert PUT semantics. Internally resolve to UUID.

Phase 2 (CodeValdFunctions, if Work contract intentionally takes UUIDs):
- Update `next-task` to lookup by slug first via a list endpoint, then call GetAgent / AssignTask with the UUID.

Phase 3 (QA docs):
- Update `09-work-01-task-create-assign.md` Step 1's example payload only if the contract changes.

## Verification

After the fix:
- `next-task` triggered with `payload={"agent_id":"developer-01"}` completes with `result.task_assigned=<TASK_ID>` and the task transitions PENDING → IN_PROGRESS.
- The Work-1 verdict's `NEW_TASK_ID` becomes non-empty.
- Pipeline proceeds to Work-2 (AI decomposition).

## Dependencies

- Discovered *after* [BUG-09-026](../../../../CodeValdCross/documentation/3-SofwareDevelopment/bug-details/BUG-09-026_http_publish_skips_fanout.md) was worked around. Fixing 026 alone leaves 027 as the next blocker.
- May be related to historical agent-creation route changes — check git log for `GetAgent` / `AssignTask` if the routes used to accept slugs.
