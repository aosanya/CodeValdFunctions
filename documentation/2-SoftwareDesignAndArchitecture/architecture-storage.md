# CodeValdFunctions — Storage

## Overview

CodeValdFunctions uses ArangoDB via `entitygraph.DataManager` (CodeValdSharedLib)
to store Job entities and agency pipeline definitions.

---

## Collections

| Collection | Type | Contents |
|---|---|---|
| `functions_entities` | Document | Jobs and FunctionsPipeline entities |
| `functions_relationships` | Edge | Reserved for future entity relationships |
| `functions_schemas_draft` | Document | Draft TypeDefinition schemas per agency |
| `functions_schemas_published` | Document | Published schema snapshots (append-only) |

Named graph: `functions_graph`.

---

## Entity Types

### Job

Tracked in `functions_entities`. See [job-lifecycle.md](../3-SofwareDevelopment/mvp-details/job-lifecycle.md)
for the full property schema and state machine.

### FunctionsPipeline

Tracked in `functions_entities`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Entitygraph ID |
| `agency_id` | string | Owning agency |
| `steps` | []Step | Event → function bindings |

See [architecture-steps.md](architecture-steps.md) for the step schema.

---

## Schema Seeding

`DefaultFunctionsSchema()` in `schema.go` is seeded into the entity graph on startup
via `entitygraph.SchemaManager`. It defines TypeDefinitions for `Job` and
`FunctionsPipeline`.

---

## CodeValdSharedLib Dependency

| Package | Usage |
|---|---|
| `entitygraph` | `DataManager` and `SchemaManager` — storage layer |
| `registrar` | Cross heartbeat (Register RPC every 20 s) |
| `serverutil` | `NewGRPCServer`, `RunWithGracefulShutdown`, `EnvOrDefault` |
| `arangoutil` | `Connect(ctx, Config)` — ArangoDB connection bootstrap |
| `gen/go/codevaldcross/v1` | Generated stubs for Cross `OrchestratorService` |
| `types` | `PathBinding`, `RouteInfo`, `ServiceRegistration` |
