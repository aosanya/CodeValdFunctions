# CodeValdFunctions — MVP Task List

## Scope

CodeValdFunctions is a general-purpose compute workhorse gRPC service for the
CodeVald platform. It runs pre-built functions against data owned by other
CodeVald services (CodeValdGit, CodeValdDT, CodeValdComm, etc.) in response
to platform events routed by CodeValdCross.

The MVP delivers:
1. A working gRPC service shell (same pattern as CodeValdGit)
2. A Job entity that tracks every function execution
3. An event subscriber that receives events from CodeValdCross and dispatches them to the right function
4. A pre-built function registry with at least one function implemented
5. A gRPC API for querying and managing jobs

A scheduler (time-triggered functions) is out of MVP scope but is architecturally designed for future addition.

---

## Tasks

### MVP-FN-001 — Service Scaffolding

**Status**: 🔲 Not Started
**Detail**: [mvp-details/platform.md](mvp-details/platform.md)

Set up the gRPC service shell following the CodeValdGit pattern:
- Per-agency service instance (agency context injected at construction time)
- CodeValdCross registration and 20-second heartbeat
- Proto definition skeleton (`FunctionsService`)
- `cmd/server/main.go` + `internal/config` + `internal/app`

**Done when**: Service starts, registers with CodeValdCross, and responds to gRPC health checks.

---

### MVP-FN-002 — Job Entity Schema

**Status**: 🔲 Not Started
**Detail**: [mvp-details/job-lifecycle.md](mvp-details/job-lifecycle.md)

Define the ArangoDB entity graph schema for the `Job` entity using
`entitygraph.SchemaManager` (same pattern as CodeValdGit's `DefaultGitSchema`).

Job properties:
| Field | Type | Description |
|---|---|---|
| `id` | string | Entitygraph ID |
| `status` | string | `pending \| running \| completed \| failed \| cancelled \| retrying` |
| `function_name` | string | Name of the pre-built function that was/is running |
| `trigger_event` | string | Full event name that triggered this job (e.g. `work.task.completed`) |
| `trigger_payload` | string | JSON-encoded event payload |
| `result` | string | JSON-encoded function output (on completion) |
| `error` | string | Error message (on failure) |
| `retry_count` | int | Number of retries attempted |
| `created_at` | timestamp | When the job was created |
| `started_at` | timestamp | When execution began |
| `completed_at` | timestamp | When execution ended (terminal state) |

**Done when**: Schema seeds correctly on startup and Job entities can be created/read via `entitygraph.DataManager`.

---

### MVP-FN-003 — Job Lifecycle & CRUD

**Status**: 🔲 Not Started
**Detail**: [mvp-details/job-lifecycle.md](mvp-details/job-lifecycle.md)

Implement the Job state machine and internal service layer:

```
pending → running → completed
                 → failed → retrying → running (loop)
                 → cancelled
```

Internal operations:
- `CreateJob(ctx, functionName, triggerEvent, payload)` → Job
- `StartJob(ctx, jobID)` → updates status to `running`, sets `started_at`
- `CompleteJob(ctx, jobID, result)` → updates status to `completed`, sets `completed_at`
- `FailJob(ctx, jobID, err)` → updates status to `failed` or `retrying`
- `CancelJob(ctx, jobID)` → updates status to `cancelled` (only from `pending` or `running`)

**Done when**: All state transitions enforced; invalid transitions return structured errors.

---

### MVP-FN-004 — Event Subscription (CodeValdCross)

**Status**: 🔲 Not Started
**Detail**: [mvp-details/event-subscription.md](mvp-details/event-subscription.md)

Subscribe to CodeValdCross and receive platform events. On each event:
1. Match the event name against the function registry
2. If a function is registered for that event, create a `Job` and dispatch

Event name format: `{service}.{entity}.{action}` — e.g., `work.task.completed`, `pubsub.topic.registered`.

**Done when**: Service receives a test event from CodeValdCross and creates a corresponding Job record.

---

### MVP-FN-005 — Function Registry & Dispatch

**Status**: 🔲 Not Started
**Detail**: [mvp-details/function-registry.md](mvp-details/function-registry.md)

A static in-process registry that maps event names to pre-built function handlers.

```go
type FunctionHandler func(ctx context.Context, job Job, payload []byte) (result []byte, err error)

type Registry interface {
    Register(eventName string, fn FunctionHandler)
    Dispatch(ctx context.Context, job Job, payload []byte) error
}
```

Functions are registered at startup (`internal/functions/init.go`). The dispatcher:
1. Looks up the handler for the job's `trigger_event`
2. Calls the handler
3. Updates job status to `completed` or `failed` based on the result

**Done when**: At least one pre-built function is registered and executes end-to-end when its trigger event fires.

---

### MVP-FN-006 — Job gRPC API

**Status**: 🔲 Not Started
**Detail**: [mvp-details/grpc-api.md](mvp-details/grpc-api.md)

Expose job management over gRPC so other services and the UI can query execution history:

| RPC | Description |
|---|---|
| `ListJobs` | Return all jobs, optionally filtered by status or function name |
| `GetJob` | Return a single job by ID |
| `CancelJob` | Cancel a pending or running job |

**Done when**: All three RPCs respond correctly; `CancelJob` honours the state machine.

---

### MVP-FN-007 — Scheduler (Future)

**Status**: 🔲 Future — not in MVP scope
**Detail**: [mvp-details/scheduler.md](mvp-details/scheduler.md)

Time-triggered function execution (cron-style). Documented for architecture
awareness but not implemented in the MVP.
