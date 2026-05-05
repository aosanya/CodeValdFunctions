# CodeValdFunctions — MVP Detail Index

## Domain Overview

CodeValdFunctions is an event-driven gRPC compute workhorse for the CodeVald platform.
It runs pre-built functions against data owned by other services in response to
platform events routed by CodeValdCross, and tracks every execution as a `Job` entity.

## Architecture Summary

```
CodeValdCross
    │  (event notifications)
    ▼
CodeValdFunctions
    ├── Event Subscriber  ──→  Function Registry  ──→  Function Handler
    │                                                        │
    └── Job Store (ArangoDB entitygraph)  ◄────────── Job lifecycle updates
```

- **Service shape**: Long-lived gRPC service, per-agency instance (CodeValdGit pattern)
- **Trigger**: Events from CodeValdCross (`{service}.{entity}.{action}` format)
- **Functions**: Pre-built, statically registered at startup
- **Storage**: ArangoDB via `entitygraph.DataManager` (CodeValdSharedLib)
- **Future**: Scheduler for time-triggered functions (MVP-FN-007)

---

## Task Index

| Task | File | Status |
|---|---|---|
| MVP-FN-001: Service Scaffolding | [platform.md](platform.md) | 🔲 Not Started |
| MVP-FN-002: Job Entity Schema | [job-lifecycle.md](job-lifecycle.md) | 🔲 Not Started |
| MVP-FN-003: Job Lifecycle & CRUD | [job-lifecycle.md](job-lifecycle.md) | 🔲 Not Started |
| MVP-FN-004: Event Subscription | [event-subscription.md](event-subscription.md) | 🔲 Not Started |
| MVP-FN-005: Function Registry & Dispatch | [function-registry.md](function-registry.md) | 🔲 Not Started |
| MVP-FN-006: Job gRPC API | [grpc-api.md](grpc-api.md) | 🔲 Not Started |
| MVP-FN-007: Scheduler (Future) | [scheduler.md](scheduler.md) | 🔲 Future |
