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
| [BUG-09-021](bug-details/BUG-09-021_imports_for_unwritten_files.md) | AI emits imports for files it never writes | Medium | 📋 Open | BUG-09-020 (for the durable compile-flutter check to be reliable) |

---

## BUG-09-021 — AI imports files it never writes

**Status**: 📋 Open · **Severity**: Medium · **Estimated effort**: ~3h (compile-flutter check) + ~30m (agency.json rule)

`main.dart` on `main` after MVP-SF-003 imports `features/dashboard/dashboard_screen.dart` and `features/dashboard/dashboard_notifier.dart` — neither file is committed. The decomposer LLM trusts its own self-consistency and doesn't verify that every `import` resolves to a `git.file.write` todo earlier in the array. `flutter analyze` returned `status=ok` only because BUG-09-020's race meant the analyzer ran against a stale main.dart.

**Durable fix (CodeValdFunctions)**: post-`flutter analyze` step in `functions/compile-flutter` — re-clone, grep `*.dart` imports, resolve against the file tree, return `issues-found` on any `uri_does_not_exist`.

**Belt-and-braces (CodeValdImplementations)**: add `RULE IMPORT-CHECK` to the decomposer prompt requiring every Dart import to match an earlier `git.file.write` todo. Cheap; LLMs occasionally ignore prompt rules.

See: [bug-details/BUG-09-021_imports_for_unwritten_files.md](bug-details/BUG-09-021_imports_for_unwritten_files.md)
