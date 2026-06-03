# CodeValdFunctions — Active Bug Backlog

## Overview

Bugs in scope for CodeValdFunctions. Mirrors the `mvp.md` / `mvp_done.md` / `mvp-details/` layout used for feature work.

- **Fixed bugs**: see [`bugs_done.md`](bugs_done.md)
- **Per-bug detail**: see [`bug-details/`](bug-details/)
- **Master cross-service queue**: [`../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md`](../../../CodeValdCross/documentation/3-SofwareDevelopment/prioritization.md)

## Workflow

### Completion Process (MANDATORY)
1. Implement and validate (`go build ./...`, `go vet ./...`, `go test -race ./...`)
2. Move the bug row from this file to `bugs_done.md`
3. Update the detail file's Status header to `✅ Fixed (YYYY-MM-DD)` and cite the commit / branch
4. Strike-through + ✅ the entry on the master prioritization.md
5. Merge feature branch to main

### Status Legend
- 📋 **Open** — not yet started or in triage
- 🚀 **In Progress** — actively being worked
- ⏸️ **Blocked** — waiting on a dependency
- ✅ **Fixed** — moved to `bugs_done.md` (do not list here)

---

## Active Bugs

| Bug ID | Title | Severity | Status | Depends On |
|--------|-------|----------|--------|------------|
| ~~[BUG-20260603-001](bug-details/BUG-20260603-001_merge-flutter-branch-fires-per-todo-not-per-task.md)~~ | ~~merge-flutter-branch fires once per completed todo instead of once per task~~ | High | ✅ Fixed 2026-06-03 — branch HEAD idempotency check added | — |
| ~~BUG-09-027~~ | ~~`next-task` 404s looking up agent by slug — fixed in CodeValdWork (GetAgent slug fallback)~~ | High | ✅ Fixed (working tree) | [BUG-09-026](../../../CodeValdCross/documentation/3-SofwareDevelopment/bug-details/BUG-09-026_http_publish_skips_fanout.md) (also fixed) |

### BUG-20260603-001 — merge-flutter-branch fires once per completed todo instead of once per task

**Severity:** High — N completed todos → N merge jobs; bloats git log with redundant merges; risks race conditions
**Status:** 📋 Open

Each `work.todo.completed` event triggers `compile-on-todo-completed` → `compile-flutter`. Each successful compile triggers `merge-on-compile-success` → `merge-flutter-branch`. With N todos, N merge jobs fire for the same branch. QA run 2026-06-03: 13 completed todos → 15 compile-flutter jobs → 15 merge-flutter-branch jobs (expected: 1 merge).

Fix: guard in `merge-flutter-branch` body — before issuing the merge, check that all todos for the task are terminal. If any are still active, exit early with a "skipped" result.

See [bug-details/BUG-20260603-001](bug-details/BUG-20260603-001_merge-flutter-branch-fires-per-todo-not-per-task.md) for fix plan.

---

### ~~BUG-09-027~~ — `next-task` function 404s on agent lookup ✅

**Severity:** High — blocked `next-task-selector`; pipeline could not start
**Detail:** [bug-details/BUG-09-027](bug-details/BUG-09-027_next_task_agent_lookup_404.md)

Fixed in CodeValdWork: `GetAgent` now accepts either UUID or `agent_id` slug; `AssignTask` writes the edge with the resolved UUID. End-to-end verified: `MVP-SF-001` was assigned to `developer-01` via the next-task-selector and transitioned to IN_PROGRESS.
