# 2 — Software Design & Architecture

## Overview

This section captures the **how** — design decisions, component architecture,
and technical constraints for CodeValdFunctions as an event-driven function
execution platform.

---

## Current Architecture (Function Execution Platform)

| Document | Description |
|---|---|
| [architecture.md](architecture.md) | High-level overview — architecture diagram, key decisions, component index, events |
| [architecture-steps.md](architecture-steps.md) | Agency step definitions — `FunctionsPipeline` entity, event matching, startup behaviour |
| [architecture-sandbox.md](architecture-sandbox.md) | Subprocess sandbox — Linux namespaces, resource caps, working directory lifecycle |
| [architecture-compiler.md](architecture-compiler.md) | Compiler workload — trigger, input fetch, binary invocation, output |
| [architecture-service-api.md](architecture-service-api.md) | `FunctionsService` gRPC API — Job query, cancel, HTTP routes |
| [architecture-storage.md](architecture-storage.md) | ArangoDB storage — collections, entity types, schema seeding |

---

## Deprecated (Copied from CodeValdGit — Do Not Use)

These files describe CodeValdFunctions when it was a git service copy. They are
retained for reference only and do not reflect the current service design.

| Document | Original Topic |
|---|---|
| [architecture-arangodb.md](architecture-arangodb.md) | ArangoDB git backend design |
| [architecture-arangodb-storer.md](architecture-arangodb-storer.md) | go-git `storage.Storer` in ArangoDB |
| [architecture-concurrency.md](architecture-concurrency.md) | Git ref locking and CAS |
| [architecture-merge.md](architecture-merge.md) | Squash merge strategy |
| [architecture-transactions.md](architecture-transactions.md) | Git transaction boundaries |
| [architecture-storer-gaps.md](architecture-storer-gaps.md) | Storer implementation gaps |
| [architecture-pull-flow.md](architecture-pull-flow.md) | Git pull/clone object serving |
| [architecture-git-http.md](architecture-git-http.md) | Git Smart HTTP transport |
| [architecture-knowledge-graph.md](architecture-knowledge-graph.md) | Knowledge graph overlay |
