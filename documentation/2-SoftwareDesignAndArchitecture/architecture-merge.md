# CodeValdFunctions — merge-flutter-branch Function

## Overview

`merge-flutter-branch` is a function binary that merges the feature branch to
`main` in CodeValdGit when `compile-flutter` reports no issues. It is a pure
service call — no AI involved — triggered by `functions.job.completed` qualified
by `payload_match`.

---

## Trigger and Payload Qualification

| Field | Value |
|---|---|
| Trigger topic | `functions.job.completed` |
| `payload_match.function_name` | `compile-flutter` |

The `payload_match` qualifier in the manifest means `BinaryRunner.Lookup` only
dispatches this function when the incoming `functions.job.completed` event
carries `"function_name":"compile-flutter"` in its JSON payload. Events where
`function_name` is anything else (including `merge-flutter-branch` itself) do
**not** match — preventing re-triggering without any guard code inside the binary.

---

## Why payload_match, not a guard inside the binary

Without `payload_match`, every `functions.job.completed` event would create a
job and run the binary. When `merge-flutter-branch` itself completes it publishes
`functions.job.completed { function_name: "merge-flutter-branch" }`, which would
re-trigger the binary, which would complete, publish again — an infinite chain.

`payload_match` resolves this at the lookup layer: no job is ever created for
non-qualifying events, so nothing re-publishes.

---

## Execution Flow

```
functions.job.completed { job_id, function_name: "compile-flutter" }
        │
BinaryRunner.Lookup("functions.job.completed", payload)
  payload_match { "function_name": "compile-flutter" } → ✓ match
        │
Job created → merge-flutter-branch binary runs
        │
1. Parse payload → extract compile job_id
2. GET /functions/{agencyId}/jobs/{compile_job_id}
   → result: { branch, task_name, status }
        │
3. Guard: branch == "main" or status != "ok" → return ok/skipped
        │
4. POST /git/{agencyId}/repositories/{repo}/branches/{branch}/merge
   { merge_into: "main", message: "Merge {branch} — flutter analyze passed" }
        │
┌───────┴──────────────────┐
│ 2xx                       │ 4xx/5xx (conflict, missing branch, etc.)
▼                           ▼
{"status":"ok"}             {"status":"error","error":"..."}
        │                           │
CompleteJob                     FailJob → retrying → failed
        │                           │
functions.job.completed         functions.job.failed
{ function_name:                { function_name:
  "merge-flutter-branch" }        "merge-flutter-branch" }
        ↓                               ↓
payload_match fails —           merge-failure-diagnostics
no function triggered           (codevaldai) diagnoses
```

---

## Manifest

`/opt/functions/merge-flutter-branch.json`:
```json
{
  "name": "merge-flutter-branch",
  "trigger": "functions.job.completed",
  "payload_match": {
    "function_name": "compile-flutter"
  }
}
```

---

## payload_match Matching Rules

`BinaryRunner.Lookup` accepts the raw event payload string alongside the topic.
`payloadMatches` checks each key-value pair in `payload_match` as the literal
substring `"key":"value"` in the JSON. Simple, no parsing overhead, works for
all string scalar fields in the published event payload.

---

## Merge Failure Diagnostics

When `merge-flutter-branch` fails, `functions.job.failed { function_name: "merge-flutter-branch" }`
is published. The `merge-failure-diagnostics` work plan fires:

- handler\_service: `codevaldai`
- payload\_condition: `"function_name":"merge-flutter-branch"`
- The AI fetches the failed job's error field, identifies the root cause
  (conflict, missing branch, permissions), and logs a diagnosis
- **No automated fix** — the developer resolves and re-triggers manually

---

## Published Events (all carry `{ job_id, function_name, trigger_event }`)

| Topic | When |
|---|---|
| `functions.job.created` | Job entity created |
| `functions.job.started` | Binary begins execution |
| `functions.job.completed` | Merge succeeded or skipped |
| `functions.job.failed` | Merge failed after retries |

> Source: `review/review.md` (March 2026 design review)
> Status: Defined — implementation tracked in `mvp.md` (GIT-012)

---

## Problem

The current `MergeBranch` implementation advances the default-branch HEAD
pointer directly to the task branch HEAD. This:

1. Does not detect whether the task branch diverged from the current default
   branch since it was created.
2. Silently overwrites any commits that landed on the default branch while the
   task was running.

The legacy go-git `repo.go` had cherry-pick rebase, but replaying each commit
individually is fragile, changes commit IDs, and is difficult to make
crash-safe.

---

## Decision: Tree-Diff Squash Merge

```
1. Read fork_point_commit_id from the Branch entity (set by CreateBranch).
2. Read current default-branch HEAD commit.
3. If fork-point == default HEAD → fast-forward: advance pointer, done.
4. If fork-point != default HEAD → diverged:
   a. Compute tree diff: fork-point tree → task HEAD tree  (agent's net changes)
   b. Apply diff to current default HEAD tree
   c. If clean → create one squash Commit entity on the default branch
   d. If conflicts → return ErrMergeConflict{Files: [...]}
```

---

## Cherry-Pick Rebase vs Squash Merge

| Property | Cherry-pick Rebase | Squash Merge |
|---|---|---|
| Commit IDs change | Yes — author/tooling confusion | No — single new commit |
| Intermediate commits replayed | Yes — noisy, multi-step | No — net-change only |
| Conflict surface | Per-commit | Per-file tree diff |
| Partial failure recovery | Hard — loop state | Simple — apply is atomic |
| Task branch history preserved | Yes | Yes — branch retained for audit |

---

## Fork-Point Tracking

`CreateBranch` must record the default-branch HEAD at creation time as
`fork_point_commit_id` on the `Branch` entity. `MergeBranch` reads this to
determine divergence.

Add to `Branch` in `models.go`:

```go
// ForkPointCommitID is the default-branch HEAD commit ID at the time this
// branch was created. Used by MergeBranch to detect divergence and compute
// the correct tree diff base.
ForkPointCommitID string `json:"fork_point_commit_id,omitempty"`
```

`CreateBranch` in `git_impl_repo.go` must populate this field when creating
a task branch:

```go
Properties: map[string]any{
    "name":                  req.Name,
    "is_default":            false,
    "head_commit_id":        sourceBranch.HeadCommitID,
    "fork_point_commit_id":  sourceBranch.HeadCommitID,  // ← new
    "created_at":            now,
    "updated_at":            now,
},
```

---

## Conflict Surface

`ErrMergeConflict` in `types.go` is already defined. For squash merge, the
`Files` field carries the paths where the agent's tree diff could not be
applied cleanly to the current default-branch tree.

```go
// ErrMergeConflict is returned by MergeBranch when the tree-diff apply
// cannot complete cleanly. The branch is left untouched; the caller is
// responsible for routing the conflict back to the agent.
type ErrMergeConflict struct {
    Files []string // conflicting file paths
}
```

---

## Branch History Retention

The task branch entity is **not** deleted by `MergeBranch`. The caller
(`DeleteBranch`) removes it after the merge succeeds. The squash commit on the
default branch stores `source_branch_id` as metadata for audit.

See [architecture-concurrency.md](architecture-concurrency.md) for the
serialisation wrapper around this operation, and
[architecture-transactions.md](architecture-transactions.md) for crash-safety
rules.
