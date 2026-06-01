# BUG-09-021 — AI emits imports for files it never writes

**Status:** 📋 Open
**Severity:** Medium — code on `main` doesn't compile in a fresh checkout
**Owner:** CodeValdFunctions (compile-flutter verification path is the durable fix); secondary fix in `CodeValdImplementations/Agencies/utility-app-builder/agency.json` decomposer prompt
**Estimated effort:** ~3h (compile-flutter import-resolution check); ~30m (agency.json prompt rule)
**Source finding:** [`/4-QA/agencies/utility-app-builder/bugs/09-mvp-sf-pipeline-findings.md`](../../../../CodeValdCross/documentation/4-QA/agencies/utility-app-builder/bugs/09-mvp-sf-pipeline-findings.md)

---

## Reproducer

`cat lib/main.dart` on `main` after the MVP-SF-003 merge.

## Evidence

After MVP-SF-003, `lib/main.dart` on `main` imports:

```dart
import 'features/dashboard/dashboard_screen.dart';
import 'features/dashboard/dashboard_notifier.dart';
```

Neither file is committed to the branch. `dashboard_notifier.dart` was in a `git.file.write` event (see BUG-09-020) but never landed on disk.

`flutter analyze` returned `status=ok` because at the time it ran, `main.dart` had not yet been overwritten by the import-laden version — it was still the simpler 13-line scaffold. The race in BUG-09-020 made compile pass incorrectly.

## Root cause hypothesis

The decomposer agent generates todos in dependency order but doesn't validate that every file referenced via `import` in a generated source file has a corresponding `git.file.write` todo. The LLM trusts its own self-consistency, which is unreliable.

## Fix locations

Ranked by durability:

### Best (CodeValdFunctions): compile-flutter import-resolution verification

Add a verification step in `functions/compile-flutter` that runs **after** `flutter analyze` succeeds:

1. Re-clone the branch.
2. Grep all `*.dart` files for `import` statements.
3. Resolve each relative import path against the file tree.
4. Return `issues-found` with a clear `uri_does_not_exist` message if any import doesn't resolve.

This catches the AI's mistake even when BUG-09-020 is also broken. Depends on a deterministic clone — once BUG-09-020 Phase 1 is fixed this becomes reliable.

### Belt-and-braces (CodeValdImplementations): decomposer rule

Add a `RULE IMPORT-CHECK` to the decomposer prompt in `agency.json`:

> Every Dart import path you emit in any file write must correspond to a `git.file.write` todo earlier in the array.

LLMs occasionally ignore rules; treat this as a cheap pre-filter, not a guarantee.

### Best long-term: trust `flutter analyze`

`flutter analyze` already detects `uri_does_not_exist`. Rely on it once BUG-09-020 is fixed and the branch is fully committed before the analyzer runs.

## Verification once fixed

- After a deliberately-malformed decomposition (e.g. main.dart imports a non-existent file), compile-flutter must return `issues-found` with the offending import listed.
- Add to /09-work-04 verdict: "every `*.dart` file on the branch parses without `uri_does_not_exist` errors against the branch's own file tree."

## Dependencies on other gaps

- The "Best" fix above depends on BUG-09-020 being fixed (so the clone reflects the full committed tree).
- Once both #20 and this are fixed, the compile gate becomes meaningful.
