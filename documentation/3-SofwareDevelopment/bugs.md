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
| BUG-09-027 | `next-task` 404s looking up agent by slug; AssignTask 404s the same way | High | 📋 Open | [BUG-09-026](../../../CodeValdCross/documentation/3-SofwareDevelopment/bug-details/BUG-09-026_http_publish_skips_fanout.md) (must be fixed first to even reach this code path) |

### BUG-09-027 — `next-task` function 404s on agent lookup

**Severity:** High — blocks `next-task-selector` from assigning any task; entire 09 Part-G pipeline cannot start
**Detail:** [bug-details/BUG-09-027](bug-details/BUG-09-027_next_task_agent_lookup_404.md)

`GET /work/{agency}/agents/developer-01` and `PUT /work/{agency}/tasks/{taskId}/assignee/developer-01` both 404 even though the upsert PUT to `/agents/developer-01` accepts the slug. Either CodeValdWork's GetAgent + AssignTask routes need to accept the slug like the upsert does, or `next-task` must resolve slug → UUID first.
