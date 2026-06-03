# CodeValdFunctions — Fixed Bugs

Bugs marked Fixed are removed from `bugs.md` and recorded here with their resolution date and the commit / branch that landed the fix.

| Bug ID | Title | Severity | Fixed Date | Commit / Branch | Detail |
|--------|-------|----------|------------|-----------------|--------|
| BUG-20260603-002 | `emit-event` and `start-pipeline` share trigger `work.pipeline.requested` with no payload guard — emit-event won alphabetically and silently no-op'd pipeline requests | High | 2026-06-03 | main (payload_match guard on emit-event.json) | [bug-details/BUG-20260603-002](bug-details/BUG-20260603-002_emit-event-start-pipeline-trigger-collision.md) |
| BUG-20260603-001 | merge-flutter-branch fires once per completed todo instead of once per task | High | 2026-06-03 | main (branch-head idempotency check in merge-flutter-branch) | [bug-details/BUG-20260603-001](bug-details/BUG-20260603-001_merge-flutter-branch-fires-per-todo-not-per-task.md) |
| BUG-09-021 | AI emits imports for files it never writes | Medium | 2026-06-01 | main (87d7d24) | [bug-details/BUG-09-021_imports_for_unwritten_files.md](bug-details/BUG-09-021_imports_for_unwritten_files.md) |
