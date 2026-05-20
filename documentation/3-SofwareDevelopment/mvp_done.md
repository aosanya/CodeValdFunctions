# CodeValdFunctions — Completed MVP Tasks

| Task ID | Title | Completed | Branch | Notes |
|---------|-------|-----------|--------|-------|
| MVP-FN-001 | Service Scaffolding | 2026-05-20 | main | gRPC shell, Cross heartbeat registrar, FunctionsService proto skeleton, health check |
| MVP-FN-002 | Job Entity Schema | 2026-05-20 | main | `DefaultFunctionsSchema()`, ArangoDB backend, Job struct, schema seed on startup |
| MVP-FN-003 | Job Lifecycle & CRUD | 2026-05-20 | main | `FunctionsManager` interface + `functionsManager` impl; CreateJob/StartJob/CompleteJob/FailJob/CancelJob; state machine enforced; 14 unit tests pass |
| MVP-FN-004 | Event Subscription | 2026-05-20 | main | `EventReceiverServer` (NotifyEvent dedup + ReceivedEvent write); `FunctionsDispatcher` creates+runs Job async; `ReceivedEventTypeDefinition("functions")` added to schema; registrar subscribes to `work.task.completed` |
| MVP-FN-005 | Function Registry & Dispatch | 2026-05-20 | main | `internal/functions`: `Registry` interface + impl; `RegisterAll` wires `work.task.completed → handleWorkTaskCompleted`; `FunctionsDispatcher` starts→completes/fails Job end-to-end; `EventReceiverServiceServer` registered in app |
| MVP-FN-006 | Job gRPC API | 2026-05-20 | main | `ListJobs`, `GetJob`, `CancelJob` RPCs in proto + server; `JobFilter` type; routes registered with Cross; 17 unit tests pass |
