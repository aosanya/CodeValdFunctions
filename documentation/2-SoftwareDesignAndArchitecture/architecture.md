# CodeValdFunctions — Architecture

## What CodeValdFunctions Is

CodeValdFunctions is an event-driven compute service for the CodeVald platform.
It executes standalone function binaries in response to platform events, tracking
each execution as a `Job` entity. Functions are independent executables discovered
from a directory on disk — any language, any runtime, hot-deployable at runtime
without restarting the service.

---

## High-Level Architecture

```
CodeValdCross
    │  (NotifyEvent RPC)
    ▼
Event Receiver
    │
    └── BinaryRunner.Lookup(triggerEvent)
              │
     ┌────────┴────────┐
     │ match found      │ no match
     ▼                 ▼
Job Created          discard
(pending)            (logged)
     │
Job Started (running)
     │
BinaryRunner.Run(ctx, binaryPath, Input)
  stdin: { job_id, agency_id, task_id, payload }
  subprocess exec (no sandbox required — binary owns its own isolation)
     │
┌────┴─────────────────────┐
│ stdout JSON parseable     │ stdout unparseable + exit ≠ 0
▼                          ▼
Output.Status              infrastructure error → FailJob
"ok"/"issues-found" → CompleteJob
"error"             → FailJob
     │
publish functions.job.completed / functions.job.failed
```

---

## Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Trigger model | Agency step definitions | Each agency configures which events run which functions |
| Execution | Standalone binary via `exec.Command` | Any language, independently deployable, no recompile needed |
| Discovery | JSON manifest sidecar per binary | Rescanned on every invocation — no restart needed for new functions |
| Hot deploy | `DeployFunction` gRPC RPC | AI agents can deploy new functions at runtime |
| Protocol | stdin JSON → stdout JSON | Simple, language-agnostic, testable with any JSON tool |
| Output | `Job.result` + completion event | Downstream services subscribe to done events |
| Storage | ArangoDB via `entitygraph.DataManager` | Consistent with platform; Jobs in `functions_entities` |
| Registration | Heartbeat to Cross every 20 s | Subscription list derived from registered step events |

---

## Micro-Binary Plugin Architecture

Each function is a standalone executable placed in the functions directory
(default `/opt/functions`). Alongside it lives a JSON manifest sidecar:

```
/opt/functions/
├── compile-flutter          # Python script (chmod +x)
├── compile-flutter.json     # manifest
├── lint-yaml                # Shell script
├── lint-yaml.json           # manifest
└── ...
```

**Manifest format** (`{name}.json`):
```json
{
  "name": "compile-flutter",
  "trigger": "work.task.completed",
  "description": "Clones the task git branch and runs flutter analyze."
}
```

**Runtime protocol:**

| | Detail |
|---|---|
| stdin | `{"job_id":"…","agency_id":"…","task_id":"…","payload":"…"}` |
| stdout | `{"status":"ok"\|"issues-found"\|"error","result":"…","error":"…"}` |
| exit 0 | stdout is parsed; status field drives job outcome |
| exit ≠ 0 + unparseable stdout | infrastructure failure → job failed |

Functions decide their own outcome via the `status` field:
- `"ok"` — success, job completes
- `"issues-found"` — analysis found problems (not an infra failure), job completes
- `"error"` — function-level error, job fails

**Hot deployment:** The `BinaryRunner` rescans manifests on every `Lookup` call,
so a new function becomes available immediately after its binary and manifest are
written to disk — no service restart required. The `DeployFunction` gRPC RPC
automates writing both files.

---

## Components

| Component | Description |
|---|---|
| Event Receiver | Receives `NotifyEvent` RPC; matches against step definitions |
| BinaryRunner | Scans manifests, executes function binaries, handles deploy |
| Job Lifecycle | Creates and transitions `Job` entities; enforces valid state machine |
| gRPC API | `FunctionsService` — Job query, cancel, and function deploy endpoints |

---

## gRPC API

| RPC | Description |
|---|---|
| `ListJobs` | List jobs for an agency, optionally filtered by status or function name |
| `GetJob` | Fetch a single job by ID |
| `CancelJob` | Cancel a pending or running job |
| `DeployFunction` | Write a new function binary + manifest to the functions directory |

`DeployFunction` is designed for AI agent use: an agent can generate a function
script and deploy it at runtime without any human intervention or service restart.

---

## Events

### Consumed (from CodeValdCross)

Derived from agency step definitions at startup. Matched against manifest triggers.

| Topic | Publisher | Default function |
|---|---|---|
| `work.task.completed` | CodeValdWork | `compile-flutter` |

### Published

All events carry `{ job_id, function_name, trigger_event }` in the payload so
subscribers can qualify on `function_name` without a follow-up API call.

| Topic | Trigger |
|---|---|
| `functions.job.created` | Job entity created |
| `functions.job.started` | Binary begins execution |
| `functions.job.completed` | Job transitions to `completed` |
| `functions.job.failed` | Job transitions to `failed` after retries exhausted |

---

## Storage

Jobs stored in ArangoDB via `entitygraph.DataManager`.
See [architecture-storage.md](architecture-storage.md).

---

## Cross URL Namespaces for Function Binaries

Function binaries that call Cross APIs must use the correct URL prefix — two
namespaces co-exist under the same base URL:

| Use | URL pattern |
|---|---|
| `git clone` / `git fetch` | `{cross_base}/{agencyId}/{repoName}` (no `/git` prefix) |
| Git REST API | `{cross_base}/git/{agencyId}/repositories/...` |
| Work REST API | `{cross_base}/work/{agencyId}/tasks/...` |

`FN_GIT_CLONE_BASE` is the bare Cross base URL (e.g. `http://codevaldcross:8081`).
Shallow clones (`--depth=1`) are not supported by the Cross git HTTP proxy.

---

## Bundled Functions

### compile-flutter

Triggered by `work.task.completed`. Clones the feature branch associated with
the task, runs `flutter analyze --no-pub`, and returns analysis output.

See [architecture-compiler.md](architecture-compiler.md).

### merge-flutter-branch

Triggered by `functions.job.completed` **qualified by**
`payload_match: { "function_name": "compile-flutter" }`. Calls the CodeValdGit
merge REST endpoint to merge the feature branch to main. Only fires when
compile-flutter passes — the `payload_match` qualifier prevents re-triggering
when merge-flutter-branch itself completes.

See [architecture-merge.md](architecture-merge.md).
