# CodeValdFunctions — Compiler Workload

## Overview

The compiler workload is a pre-built function that compiles the source files written
to a task's git branch. It is the first concrete function handler registered in
CodeValdFunctions and serves as the reference implementation for the handler pattern.

---

## Trigger

| Event | Publisher | Payload field used |
|---|---|---|
| `work.task.completed` | CodeValdWork | `task_id` |

---

## Execution Flow

The handler runs in two distinct phases: a **pre-sandbox phase** (network allowed,
used to fetch inputs and download dependencies) and a **sandbox phase** (network
disabled, filesystem isolated, resource capped).

```
work.task.completed
        │
CompilerHandler(ctx, job, payload)
        │
── PRE-SANDBOX PHASE (network allowed) ────────────────────────
        │
1. Extract task_id from payload
        │
2. Resolve git branch via Cross
   GET /{agencyId}/repositories  →  repo.ID
   GET /{agencyId}/branches?name=task/{task-id}  →  branch.ID
        │
3. List all files on branch
   GET /{agencyId}/branches/{branchId}/tree
        │
4. Download each file via Cross → materialise into
   /tmp/fn-{job-id}/src/{file-paths}
        │
5. If go.mod present: run go mod download (unsandboxed)
   GOMODCACHE=/tmp/fn-{job-id}/modcache go mod download
   (populates module cache without vendor/ required)
        │
── SANDBOX PHASE (CLONE_NEWNET + CLONE_NEWPID + CLONE_NEWNS) ──
        │
6. sandbox.Run:
   WorkDir: /tmp/fn-{job-id}/src/
   Binary:  go
   Args:    [build, -mod=mod, ./...]
   Env:     GOMODCACHE=/tmp/fn-{job-id}/modcache
            GOFLAGS=-mod=mod
        │
── CLEANUP (always) ────────────────────────────────────────────
        │
7. os.RemoveAll(/tmp/fn-{job-id}/)
        │
┌───────┴───────┐
│ exit 0         │ exit ≠ 0
▼               ▼
Job.result=stdout  Job.error=stderr
        │               │
functions.job.completed  functions.job.failed
```

---

## Agency Step Definition

```json
{
  "trigger_event": "work.task.completed",
  "function": "compile-go"
}
```

---

## Supported Handlers

| Handler Name | Binary | Args | Language |
|---|---|---|---|
| `compile-go` | `go` | `build ./...` | Go |

Additional language handlers follow the same pattern and are added as separate
files under `internal/functions/`. Each handler is registered in
`internal/functions/init.go`.

---

## Input Fetching Detail

The handler uses the Cross HTTP client to walk the git branch:

1. `GET /{agencyId}/repositories` — find the agency's repository
2. `GET /{agencyId}/branches` with `?name=task/{task-id}` — resolve the branch
3. `GET /{agencyId}/branches/{branchId}/tree` — list all file paths recursively
4. `GET /{agencyId}/branches/{branchId}/files/{path}` — download each file

Files are written to the temp dir preserving directory structure relative to the
repository root.

---

## Output

| Outcome | Stored on Job | Event published |
|---|---|---|
| Compiler exit 0 | `result` = stdout | `functions.job.completed` |
| Compiler exit ≠ 0 | `error` = stderr (+ stdout if non-empty) | `functions.job.failed` |
| Input fetch error | `error` = error message | `functions.job.failed` |
| Timeout / cancel | `error` = "context deadline exceeded" | `functions.job.failed` |

---

## Module Dependency Resolution

`go build` requires all dependencies to be available before the sandbox's network
namespace is created. The handler resolves this with a two-phase approach:

| Phase | Network | What happens |
|---|---|---|
| Pre-sandbox | Allowed | `go mod download` populates `GOMODCACHE` from the internet |
| Sandbox | Disabled | `go build -mod=mod` reads from the pre-fetched `GOMODCACHE` |

`GOMODCACHE` is set to a subdirectory of the job's temp dir (`/tmp/fn-{job-id}/modcache`)
so it is isolated per job and cleaned up together with the source files. No vendoring
required; no persistent module cache on the host.

If `go.mod` is absent from the branch, the pre-fetch step is skipped and `-mod=mod`
is replaced with `-mod=vendor` to produce a clear error if dependencies are missing.

---

## Sandbox Configuration

| Setting | Value |
|---|---|
| Binary | `go` (must be on PATH in the host environment) |
| CPU limit | 60 s (longer than default to allow large repos) |
| Memory limit | 1 GB |
| File size limit | 500 MB (compiled output may be large) |
| Network | Disabled (`CLONE_NEWNET`) — modules pre-fetched before sandbox |
| Env overrides | `GOMODCACHE`, `GOFLAGS=-mod=mod` |

---

## Deployment Requirement

`go` must be installed in the CodeValdFunctions host environment and on `PATH`.
Deployment docs must capture this as a hard dependency.
