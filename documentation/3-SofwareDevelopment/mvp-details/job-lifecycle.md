# Job Lifecycle — MVP-FN-002 & MVP-FN-003

## Job Entity

The `Job` entity is the single core entity owned by CodeValdFunctions. It tracks
one execution of a pre-built function that was triggered by a platform event.

Stored in ArangoDB via `entitygraph.DataManager` (CodeValdSharedLib), in the
`functions_entities` collection (mutable entities) using the `functions_graph`
named graph.

### Properties

| Field | Type | Description |
|---|---|---|
| `id` | string | Entitygraph ID |
| `status` | string | Current state (see state machine below) |
| `function_name` | string | Name of the pre-built function |
| `trigger_event` | string | Event that triggered this job, e.g. `work.task.completed` |
| `trigger_payload` | string | JSON-encoded event payload received from CodeValdCross |
| `result` | string | JSON-encoded function output (set on `completed`) |
| `error` | string | Error message (set on `failed`) |
| `retry_count` | int | Number of retries attempted so far |
| `created_at` | timestamp | When the job record was created |
| `started_at` | timestamp | When execution began (transition to `running`) |
| `completed_at` | timestamp | When execution reached a terminal state |

---

## State Machine

```
               ┌─────────┐
               │ pending │
               └────┬────┘
                    │ StartJob
                    ▼
               ┌─────────┐
               │ running │
               └────┬────┘
          ┌─────────┼─────────┐
          │         │         │
          ▼         ▼         ▼
     completed   failed   cancelled
                    │
                    │ (retry eligible)
                    ▼
               ┌──────────┐
               │ retrying │
               └────┬─────┘
                    │ StartJob
                    ▼
               ┌─────────┐
               │ running │  (loop)
               └─────────┘
```

### Valid Transitions

| From | To | Trigger |
|---|---|---|
| `pending` | `running` | `StartJob` |
| `pending` | `cancelled` | `CancelJob` |
| `running` | `completed` | `CompleteJob` |
| `running` | `failed` | `FailJob` (no retries left) |
| `running` | `retrying` | `FailJob` (retries remaining) |
| `running` | `cancelled` | `CancelJob` |
| `retrying` | `running` | `StartJob` (next attempt) |

Any other transition returns `FAILED_PRECONDITION`.

---

## Internal Service Layer

```go
type JobService interface {
    CreateJob(ctx context.Context, functionName, triggerEvent string, payload []byte) (Job, error)
    StartJob(ctx context.Context, jobID string) (Job, error)
    CompleteJob(ctx context.Context, jobID string, result []byte) (Job, error)
    FailJob(ctx context.Context, jobID string, err error) (Job, error)
    CancelJob(ctx context.Context, jobID string) (Job, error)
    GetJob(ctx context.Context, jobID string) (Job, error)
    ListJobs(ctx context.Context, filter JobFilter) ([]Job, error)
}
```

---

## MVP-FN-002 Acceptance Criteria

- [ ] `DefaultFunctionsSchema()` seeds Job TypeDefinition on startup
- [ ] Job entity can be created and read via `entitygraph.DataManager`

## MVP-FN-003 Acceptance Criteria

- [ ] All valid state transitions enforced
- [ ] Invalid transitions return `FAILED_PRECONDITION`
- [ ] `started_at` set on `running`, `completed_at` set on terminal states
- [ ] `retry_count` increments on each `retrying` transition
