# CodeValdFunctions — Architecture

## What CodeValdFunctions Is

CodeValdFunctions is an event-driven compute service for the CodeVald platform.
It executes pre-built function handlers in response to platform events, tracking
each execution as a `Job` entity. Functions run external tools in a sandboxed
subprocess with full network, filesystem, and resource isolation.

---

## High-Level Architecture

```
CodeValdCross
    │  (NotifyEvent RPC)
    ▼
Event Receiver
    │
    └── match against agency step definitions
              │
     ┌────────┴────────┐
     │ step found       │ no match
     ▼                 ▼
Job Created          discard
(pending)            (logged)
     │
Job Started (running)
     │
Input Resolver
(fetch inputs via Cross)
     │
Temp Dir Created
     │
Sandbox Launcher
exec.Command — Linux namespaces + RLIMIT
(no network, restricted FS, bounded resources)
     │
┌────┴────┐
│ exit 0  │ exit ≠ 0
▼         ▼
CompleteJob  FailJob
result=stdout  error=stderr
     │         │
publish  functions.job.completed / functions.job.failed
```

---

## Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Trigger model | Agency step definitions | Each agency configures which events run which functions |
| Execution | Named binary via `exec.Command` | No containers; each handler hardcodes its tool invocation |
| Sandbox | Linux namespaces + `RLIMIT` | Full isolation, no external runtime dependency |
| Input resolution | All service calls via CodeValdCross | Maintains platform routing; no direct service-to-service |
| Working directory | Temp-per-job, deleted after run | Stateless; no cross-job contamination |
| Output | `Job.result` + completion event | Simple; downstream services subscribe to done events |
| Storage | ArangoDB via `entitygraph.DataManager` | Consistent with platform; Jobs in `functions_entities` |
| Registration | Heartbeat to Cross every 20 s | Subscription list derived from registered step events |

---

## Components

| Component | Description | Detail |
|---|---|---|
| Event Receiver | Receives `NotifyEvent` RPC; matches against step definitions | [event-subscription.md](../3-SofwareDevelopment/mvp-details/event-subscription.md) |
| Agency Step Definitions | Agency pipeline: list of `{ trigger_event, function }` bindings | [architecture-steps.md](architecture-steps.md) |
| Job Lifecycle | Creates and transitions `Job` entities; enforces valid state machine | [job-lifecycle.md](../3-SofwareDevelopment/mvp-details/job-lifecycle.md) |
| Function Registry | Static in-process map of handler names to implementations | [function-registry.md](../3-SofwareDevelopment/mvp-details/function-registry.md) |
| Input Resolver | Each handler fetches its own inputs via the Cross HTTP client | — |
| Sandbox Launcher | Wraps `exec.Command` with namespace isolation and resource caps | [architecture-sandbox.md](architecture-sandbox.md) |
| gRPC API | `FunctionsService` — Job query and cancel endpoints | [architecture-service-api.md](architecture-service-api.md) |

---

## Events

### Consumed (from CodeValdCross)

Derived from the agency's step definitions at startup. For the compiler workload:

| Topic | Publisher | Step |
|---|---|---|
| `work.task.completed` | CodeValdWork | Compiler function |

### Published

| Topic | Trigger |
|---|---|
| `functions.job.completed` | Job transitions to `completed` |
| `functions.job.failed` | Job transitions to `failed` |

---

## Storage

Jobs and pipeline definitions stored in ArangoDB via `entitygraph.DataManager`.
See [architecture-storage.md](architecture-storage.md).

---

## Workloads

### Compiler

Triggered by `work.task.completed`. Fetches files from the `task/{task-id}` git
branch via CodeValdGit (through Cross), materialises them into a temp directory,
runs the compiler binary under sandbox isolation, and publishes the result.

See [architecture-compiler.md](architecture-compiler.md).
