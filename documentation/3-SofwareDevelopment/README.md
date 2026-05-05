# 3 — Software Development

## Overview

This section tracks the development plan, MVP task breakdown, and implementation
details for CodeValdFunctions.

---

## Index

| Document | Description |
|---|---|
| [mvp.md](mvp.md) | Full MVP scope, task list, and completion status |
| [mvp-details/](mvp-details/README.md) | Per-topic task specifications grouped by domain |

---

## MVP Status

| Task ID | Title | Status |
|---|---|---|
| MVP-FN-001 | Service Scaffolding | 🔲 Not Started |
| MVP-FN-002 | Job Entity Schema | 🔲 Not Started |
| MVP-FN-003 | Job Lifecycle & CRUD | 🔲 Not Started |
| MVP-FN-004 | Event Subscription (CodeValdCross) | 🔲 Not Started |
| MVP-FN-005 | Function Registry & Dispatch | 🔲 Not Started |
| MVP-FN-006 | Job gRPC API | 🔲 Not Started |
| MVP-FN-007 | Scheduler | 🔲 Future |

---

## Execution Order

```
MVP-FN-001 (Scaffolding)
    ↓
MVP-FN-002 (Job Schema)
    ↓
MVP-FN-003 (Job Lifecycle)
    ↓
MVP-FN-004 (Event Subscription) ──── MVP-FN-006 (gRPC API)
    ↓
MVP-FN-005 (Function Registry)

MVP-FN-007 (Scheduler) — future, independent track
```

---

## Task Detail Files

| File | Tasks |
|---|---|
| [mvp-details/platform.md](mvp-details/platform.md) | MVP-FN-001 |
| [mvp-details/job-lifecycle.md](mvp-details/job-lifecycle.md) | MVP-FN-002, MVP-FN-003 |
| [mvp-details/event-subscription.md](mvp-details/event-subscription.md) | MVP-FN-004 |
| [mvp-details/function-registry.md](mvp-details/function-registry.md) | MVP-FN-005 |
| [mvp-details/grpc-api.md](mvp-details/grpc-api.md) | MVP-FN-006 |
| [mvp-details/scheduler.md](mvp-details/scheduler.md) | MVP-FN-007 (future) |
