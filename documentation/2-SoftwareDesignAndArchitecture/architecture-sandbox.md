# CodeValdFunctions — Subprocess Sandbox

## Overview

Every external tool invocation runs inside a sandboxed subprocess. The sandbox
provides three independent isolation layers with no external runtime dependency
(no Docker, no OCI runtime):

1. **Network isolation** — subprocess cannot make outbound network calls
2. **Filesystem isolation** — subprocess is confined to its temp working directory
3. **Resource caps** — CPU time, memory, and file size are bounded

All layers are implemented via `syscall.SysProcAttr` and `syscall.Setrlimit`
in the `internal/sandbox/` package.

---

## Linux Namespace Isolation

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWNET | // new network namespace — no connectivity
                syscall.CLONE_NEWPID | // new PID namespace — isolated process tree
                syscall.CLONE_NEWNS,   // new mount namespace — for chroot
    Pdeathsig: syscall.SIGKILL,        // kill child if parent dies
}
```

| Namespace | Flag | Effect |
|---|---|---|
| Network | `CLONE_NEWNET` | Subprocess has a loopback-only network stack; no external hosts reachable |
| PID | `CLONE_NEWPID` | Subprocess sees itself as PID 1; cannot signal host processes |
| Mount | `CLONE_NEWNS` | Mount namespace isolated; chroot to temp dir restricts filesystem access |

---

## Resource Limits

Applied via `syscall.Setrlimit` before `cmd.Start()`:

| Resource | Default Limit | Constant |
|---|---|---|
| CPU time | 30 s | `RLIMIT_CPU` |
| Virtual memory | 512 MB | `RLIMIT_AS` |
| Max file size | 100 MB | `RLIMIT_FSIZE` |
| Open file descriptors | 64 | `RLIMIT_NOFILE` |

Limits are configurable per function handler at registration time.

---

## Working Directory Lifecycle

```
CreateTempDir("/tmp/fn-{job-id}/")
        │
materialise input files into temp dir
        │
cmd.Dir = tempDir
cmd.Run()  (sandboxed)
        │
┌───────┴───────┐
│ exit 0         │ exit ≠ 0
▼               ▼
capture stdout  capture stderr
→ Job.result    → Job.error
        │
os.RemoveAll(tempDir)   ← always runs (defer)
```

The temp directory is always removed — on success, failure, and context cancellation.

---

## Stdout / Stderr Capture

```go
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr
```

- **Exit 0**: `stdout.String()` stored as `Job.result`
- **Non-zero exit**: `stderr.String()` stored as `Job.error`; stdout appended when non-empty

---

## Context Cancellation

`exec.CommandContext(ctx, binary, args...)` is used so that context cancellation
(e.g. from a `CancelJob` call or job timeout) sends `SIGKILL` to the subprocess.
`CLONE_NEWPID` ensures the signal reaches all child processes spawned by the tool.

---

## Pre-Sandbox Steps

Some handlers require unsandboxed preparation before entering the sandbox (e.g.
downloading Go module dependencies). These steps run with normal network access.
The sandbox is entered only for the tool invocation itself.

```
handler:
  1. Materialise inputs          (no sandbox — network allowed for Cross calls)
  2. Pre-sandbox setup           (no sandbox — e.g. go mod download)
  3. sandbox.Run(...)            (sandboxed — CLONE_NEWNET blocks all outbound traffic)
  4. Cleanup                     (always, regardless of sandbox outcome)
```

---

## Sandbox Interface

```go
// Launcher runs a command in a sandboxed subprocess.
type Launcher interface {
    // Run executes binary with args in a sandboxed subprocess under workDir.
    // Returns stdout on exit 0, or an error wrapping stderr on non-zero exit.
    Run(ctx context.Context, cfg RunConfig) (stdout string, err error)
}

type RunConfig struct {
    WorkDir string            // temp directory; must exist
    Binary  string            // executable name (resolved via PATH)
    Args    []string          // command-line arguments
    Env     []string          // additional environment variables ("KEY=VALUE")
                              // merged with a minimal safe base env (PATH, HOME, TMPDIR)
    Limits  ResourceLimits
}

type ResourceLimits struct {
    CPUSecs    int // RLIMIT_CPU
    MemoryMB   int // RLIMIT_AS  (megabytes)
    FileSizeMB int // RLIMIT_FSIZE
}
```
