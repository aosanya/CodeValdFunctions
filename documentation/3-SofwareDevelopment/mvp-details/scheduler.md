# Scheduler — MVP-FN-007 (Future)

> **Status: Not in MVP scope.** Documented here for architectural awareness.

---

## Purpose

The scheduler allows functions to be triggered on a time-based schedule (cron-style)
rather than by a platform event. This is the second trigger model for CodeValdFunctions
after event-driven execution.

---

## Planned Design

A `ScheduledFunction` entity (future, not yet in MVP schema) would hold:

| Field | Description |
|---|---|
| `id` | Entity ID |
| `function_name` | Pre-built function to execute |
| `cron_expression` | Standard cron schedule (e.g. `0 * * * *`) |
| `enabled` | Whether the schedule is active |
| `last_run_at` | Timestamp of last execution |
| `next_run_at` | Timestamp of next scheduled execution |

The scheduler would run as an internal goroutine, polling for due schedules and
creating Jobs the same way the event subscriber does — going through the same
`FunctionRegistry.Dispatch` path.

---

## Why Deferred

Event-driven execution covers all current use cases. Adding the scheduler requires
a reliable distributed clock (to avoid double-firing across agency service restarts)
which adds complexity not justified at MVP scale.

---

## Future Acceptance Criteria (placeholder)

- [ ] `ScheduledFunction` entity schema defined
- [ ] gRPC API: `CreateSchedule`, `ListSchedules`, `DeleteSchedule`, `EnableSchedule`, `DisableSchedule`
- [ ] Scheduler goroutine fires functions within 60 seconds of their scheduled time
- [ ] No double-firing on service restart
