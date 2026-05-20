# Function Registry & Dispatch — MVP-FN-005

## Overview

The function registry is a static in-process map of event names to pre-built
function handlers. Functions are registered at startup in `internal/functions/`.
The registry is not persisted — it is rebuilt from code on every startup.

---

## Registry Interface

```go
// FunctionHandler is the signature all pre-built functions must implement.
type FunctionHandler func(ctx context.Context, job Job, payload []byte) (result []byte, err error)

// Registry maps event names to their handlers.
type Registry interface {
    // Register binds a pre-built function to one or more event names.
    Register(eventName string, fn FunctionHandler)
    // Lookup returns the handler for the given event name, or nil if none registered.
    Lookup(eventName string) FunctionHandler
    // RegisteredEvents returns all event names that have a handler.
    RegisteredEvents() []string
}
```

---

## Dispatch Flow

When the event subscriber receives an event:

1. `registry.Lookup(event.Name)` — find the handler
2. `jobService.CreateJob(...)` — create Job in `pending`
3. `jobService.StartJob(...)` — transition to `running`
4. `handler(ctx, job, payload)` — execute the function
5. On success: `jobService.CompleteJob(...)` with the result
6. On error: `jobService.FailJob(...)` — transitions to `failed` or `retrying`

---

## Pre-Built Functions

Functions are registered in `internal/functions/init.go`:

```go
func Register(r Registry) {
    r.Register("work.task.completed", handlers.CompileGoRepo)
}
```

Each function lives in its own file under `internal/functions/`.

### `compile-go` — Go Compiler (`internal/functions/compile_go.go`)

Triggered by `work.task.completed`. Compiles the source files written to the
task's git branch (`task/{task-id}`) via CodeValdGit (through Cross).

**Trigger**: `work.task.completed`
Note: `work.task.completed` now fires only after ALL child sub-tasks complete
(fixed in CodeValdWork `event_dispatcher.go`). This ensures the compiler sees
the full branch, not a partial state mid-decomposition.

**Flow**:
1. Extract `task_id` from payload
2. Resolve `task/{task-id}` branch in CodeValdGit via Cross
3. Download all files from the branch into `/tmp/fn-{job-id}/src/`
4. Run `go mod download` (outside sandbox) → populate `/tmp/fn-{job-id}/modcache/`
5. Run `go build -mod=mod ./...` inside Linux namespace sandbox
6. Store stdout/stderr on `Job.result` / `Job.error`
7. Publish `functions.job.completed` or `functions.job.failed`

See [architecture-compiler.md](../../../../documentation/2-SoftwareDesignAndArchitecture/architecture-compiler.md) for full detail.

---

## MVP-FN-005 Acceptance Criteria

- [ ] Registry initialises at startup with at least one function registered
- [ ] `RegisteredEvents()` returns the correct event list (used by registrar for subscriptions)
- [ ] Dispatcher creates, starts, completes, and fails jobs correctly
- [ ] At least one end-to-end test: trigger event → job created → function runs → job completed
