# CodeValdFunctions — compile-flutter Function

## Overview

`compile-flutter` is a bundled function binary that clones the git branch
associated with a completed task and runs `flutter analyze` to check for
compilation and analysis errors. It is the default function registered in the
utility-app-builder agency and serves as the reference implementation for the
micro-binary plugin architecture.

---

## Trigger

| Event | Publisher | Payload field used |
|---|---|---|
| `work.task.completed` | CodeValdWork | `task_id` |

---

## Manifest

`/opt/functions/compile-flutter.json`:
```json
{
  "name": "compile-flutter",
  "trigger": "work.task.completed",
  "description": "Clones the task git branch and runs flutter analyze to check for compilation errors."
}
```

---

## Execution Flow

```
work.task.completed
        │
BinaryRunner.Lookup("work.task.completed") → compile-flutter
        │
BinaryRunner.Run(ctx, "/opt/functions/compile-flutter", Input)
        │
stdin: { "job_id": "…", "agency_id": "…", "task_id": "…", "payload": "…" }
        │
── INSIDE compile-flutter (Python script) ──────────────────────
        │
1. Parse stdin JSON, extract task_id and agency_id
        │
2. git clone --depth 1 -b task/{task_id} <GIT_CLONE_BASE>/{agency_id}/repo /tmp/fn-{job_id}/src
        │
3. flutter analyze --no-pub /tmp/fn-{job_id}/src
        │
4. Collect stdout/stderr from flutter analyze
        │
┌───────┴───────────────────────────┐
│ No issues found                   │ Issues found
▼                                   ▼
stdout: {"status":"ok",             stdout: {"status":"issues-found",
         "result":"<output>"}                "result":"<analysis output>"}
exit 0                              exit 0
```

Analysis errors are **not** treated as infrastructure failures — they are returned
as `status: "issues-found"` so the job completes and downstream subscribers can
act on the analysis output. Only a crash or unparseable output constitutes an
infrastructure failure.

---

## Input / Output Protocol

**Input (stdin JSON):**
```json
{
  "job_id": "abc123",
  "agency_id": "utility-app-builder",
  "task_id": "task-456",
  "payload": "{\"task_id\":\"task-456\",\"terminal_status\":\"completed\"}"
}
```

**Output (stdout JSON):**
```json
{
  "status": "ok",
  "result": "Analyzing... No issues found!"
}
```

Or when issues are found:
```json
{
  "status": "issues-found",
  "result": "Analyzing...\n  error • lib/main.dart:10:5 • Undefined name 'foo'"
}
```

---

## Agency Step Definition

```json
{
  "code": "compile-on-task-completed",
  "trigger_topic": "work.task.completed",
  "function_code": "compile-flutter",
  "handler_service": "codevaldfunction"
}
```

---

## Cross URL Namespaces

Two distinct URL prefixes are used when communicating with Cross from a function binary:

| Use | URL pattern | Example |
|---|---|---|
| `git clone` / `git fetch` | `{cross_base}/{agencyId}/{repoName}` | `http://codevaldcross:8081/utility-app-builder/shared-farms` |
| REST API (list branches, get task, etc.) | `{cross_base}/git/{agencyId}/...` | `http://codevaldcross:8081/git/utility-app-builder/repositories/...` |
| Work API | `{cross_base}/work/{agencyId}/...` | `http://codevaldcross:8081/work/utility-app-builder/tasks/...` |

The git smart HTTP routes (`/info/refs`, `/git-upload-pack`) are registered by CodeValdGit
**without** the `/git/` prefix. Any function that needs to `git clone` must use
`{FN_GIT_CLONE_BASE}/{agencyId}/{repoName}` where `FN_GIT_CLONE_BASE` is the bare
Cross base URL (e.g. `http://codevaldcross:8081`), not `http://codevaldcross:8081/git`.

`--depth=1` (shallow clone) is **not supported** by the Cross git HTTP proxy — use a
full clone.

---

## Environment Requirements

| Requirement | Where provided |
|---|---|
| Python 3 | Installed in the runtime Docker image |
| Flutter SDK | Installed at `/opt/flutter` in the runtime Docker image |
| Git | Installed in the runtime Docker image |
| `GIT_CLONE_BASE` env | Set via `FN_GIT_CLONE_BASE` in docker-compose |

---

## Output Job Fields

| Outcome | `Job.status` | `Job.result` | `Job.error` |
|---|---|---|---|
| No issues | `completed` | flutter analyze stdout | — |
| Issues found | `completed` | flutter analyze output | — |
| Infrastructure failure | `failed` | — | error message |

---

## Hot Replacement

Because `compile-flutter` is a standalone binary, it can be replaced at runtime:
- A new version can be deployed by writing a new binary to `/opt/functions/compile-flutter`
- Or an AI agent can call the `DeployFunction` gRPC RPC with the updated binary bytes
- No service restart required — the next event dispatch picks up the new binary
