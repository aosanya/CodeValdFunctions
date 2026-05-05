# Event Subscription — MVP-FN-004

## Overview

CodeValdFunctions subscribes to CodeValdCross to receive platform events. On each
event it checks the function registry and, if a handler is registered, creates a
Job and dispatches it.

---

## Event Format

All platform events follow the naming convention:

```
{service}.{entity}.{action}
```

Examples from known publishers:

| Event Name | Publisher |
|---|---|
| `work.task.created` | CodeValdWork |
| `work.task.updated` | CodeValdWork |
| `work.task.status.changed` | CodeValdWork |
| `work.task.completed` | CodeValdWork |
| `work.task.assigned` | CodeValdWork |
| `work.relationship.created` | CodeValdWork |
| `pubsub.event.recorded` | CodeValdPubSub |
| `pubsub.topic.registered` | CodeValdPubSub |
| `pubsub.subscription.created` | CodeValdPubSub |

---

## Subscription Model

CodeValdFunctions declares its event subscriptions at registration time (startup).
CodeValdCross routes matching events to this service instance.

The subscriber loop:

```
CodeValdCross  →  event notification  →  CodeValdFunctions
                                               │
                                     registry.Lookup(event.Name)
                                               │
                                    ┌──────────┴──────────┐
                                    │ handler found        │ no handler
                                    ▼                      ▼
                               CreateJob              discard (log only)
                                    │
                               StartJob + Dispatch
```

---

## MVP-FN-004 Acceptance Criteria

- [ ] Subscriptions declared at registration time for all events with registered handlers
- [ ] On receiving a subscribed event, a Job is created with status `pending`
- [ ] Job is immediately dispatched (status moves to `running`)
- [ ] Unhandled events are logged and discarded without creating a Job
- [ ] Subscriber reconnects automatically if CodeValdCross connection drops
