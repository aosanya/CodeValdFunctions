# CodeValdFunctions — Agency Step Definitions

## Overview

The **agency step definition** (called a `FunctionsPipeline`) is the per-agency
configuration that maps platform events to function handlers. It is the agency's
compute pipeline — a list of steps, each binding a trigger event to a named function.

---

## Step Shape

```go
type Step struct {
    // TriggerEvent is the platform event topic that activates this step.
    // Format: {service}.{entity}.{action}  e.g. "work.task.completed"
    TriggerEvent string `json:"trigger_event"`
    // Function is the name of the pre-built function handler to invoke.
    // Must match a handler registered in internal/functions/.
    Function string `json:"function"`
}
```

---

## FunctionsPipeline Entity

Stored as a `FunctionsPipeline` entity in `functions_entities`:

| Field | Type | Description |
|---|---|---|
| `id` | string | Entitygraph ID |
| `agency_id` | string | The owning agency |
| `steps` | []Step | Ordered list of event → function bindings |

An agency has at most one `FunctionsPipeline`. Multiple steps may share the same
trigger event — each matching step creates an independent Job.

---

## Startup Behaviour

1. CodeValdFunctions loads the `FunctionsPipeline` entity for the agency.
2. All unique `TriggerEvent` values are extracted as the subscription list.
3. The registrar heartbeat declares these subscriptions to CodeValdCross every 20 s.
4. CodeValdCross routes matching events to this service instance.

---

## Event Matching

When CodeValdCross delivers a `NotifyEvent` RPC:

1. Load the agency's step list (in-memory cache, refreshed on each heartbeat).
2. Filter steps where `step.TriggerEvent == event.Topic`.
3. For each matching step: look up the function handler, create and dispatch a Job.
4. If no steps match: log the event and discard.

---

## Example Pipeline

```json
{
  "steps": [
    { "trigger_event": "work.task.completed", "function": "compile-go" }
  ]
}
```

---

## Research Gap

📝 Whether the `FunctionsPipeline` entity is created via a gRPC admin endpoint or
seeded from a startup config is not yet decided. For v1, seeding from an environment
variable or a config file at startup is simpler.
