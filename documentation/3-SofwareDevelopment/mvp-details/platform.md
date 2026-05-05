# Platform Architecture — MVP-FN-001

## Service Shape

CodeValdFunctions follows the exact same service pattern as CodeValdGit:

| Aspect | Detail |
|---|---|
| Runtime | Long-lived gRPC server |
| Agency scope | One instance per agency, agency context injected at construction |
| Registration | Heartbeat to CodeValdCross every 20 seconds |
| Config | Environment variables only (`internal/config`) |
| Entry point | `cmd/server/main.go` → `internal/app.Run(cfg)` |

## Package Layout

```
CodeValdFunctions/
├── cmd/
│   ├── server/main.go        — production entry point
│   └── dev/main.go           — local dev entry point
├── internal/
│   ├── app/                  — wires dependencies, starts gRPC server
│   ├── config/               — reads env vars
│   ├── registrar/            — CodeValdCross registration + heartbeat
│   ├── functions/            — pre-built function handlers (one file per function)
│   └── service/              — gRPC handler implementations
├── proto/
│   └── codevaldfunction/v1/
│       └── service.proto     — FunctionsService definition
├── gen/                      — generated gRPC/protobuf code
└── storage/                  — entitygraph adapter (collections, graph name)
```

## CodeValdCross Registration

On startup the service calls CodeValdCross to register its gRPC routes and event
subscriptions. The registrar then sends a heartbeat every 20 seconds to keep the
registration alive. If the heartbeat lapses, CodeValdCross stops routing events
to this instance.

## MVP-FN-001 Acceptance Criteria

- [ ] Service starts and passes gRPC health check
- [ ] Service registers with CodeValdCross on startup
- [ ] Heartbeat fires every 20 seconds and is visible in CodeValdCross logs
- [ ] `cmd/dev/main.go` works for local development
