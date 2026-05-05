# CodeValdFunctions — Documentation

## Overview

**CodeValdFunctions** is a Go gRPC microservice that provides a general-purpose
compute workhorse for the CodeVald platform. It runs pre-built functions against
data owned by other CodeVald services (CodeValdGit, CodeValdDT, CodeValdComm, etc.)
in response to platform events or (future) scheduled triggers.

It is scoped per-agency at construction time — the same pattern as CodeValdGit.

---

## Documentation Index

| Document | Description |
|---|---|
| [1-SoftwareRequirements/](1-SoftwareRequirements/README.md) | Scope, functional requirements, NFR |
| [2-SoftwareDesignAndArchitecture/](2-SoftwareDesignAndArchitecture/README.md) | Architecture decisions, entity schema, event model |
| [3-SofwareDevelopment/](3-SofwareDevelopment/README.md) | MVP task list, implementation details per topic |
| [4-QA/](4-QA/README.md) | Testing strategy, acceptance criteria |

---

## Quick Summary

- **Language**: Go
- **Service type**: Long-lived gRPC service (same pattern as CodeValdGit)
- **Consumer**: Platform services via CodeValdCross HTTP proxy
- **Agency scope**: One service instance per agency
- **Trigger model**: Event-driven (CodeValdCross subscriber); scheduler planned for future
- **Core entity**: `Job` — tracks a function execution triggered by a platform event
- **Function model**: Pre-built functions, statically registered, extensible over time
