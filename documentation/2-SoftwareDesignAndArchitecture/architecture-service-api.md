# CodeValdFunctions — Service API

## FunctionsService gRPC

The `FunctionsService` exposes Job query and management endpoints consumed by
CodeValdCross-proxied callers. The internal dispatch loop (event → job creation
→ function execution) is not exposed via gRPC — it runs autonomously.

---

## Interface

```go
service FunctionsService {
    // GetJob returns a single Job by ID.
    rpc GetJob(GetJobRequest) returns (GetJobResponse);
    // ListJobs returns jobs for the agency with optional filter.
    rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);
    // CancelJob cancels a pending or running job.
    rpc CancelJob(CancelJobRequest) returns (CancelJobResponse);
}
```

---

## HTTP Routes (via CodeValdCross)

| Method | Path | gRPC Method | Notes |
|---|---|---|---|
| `GET` | `/{agencyId}/jobs/{jobId}` | `GetJob` | Returns Job entity |
| `GET` | `/{agencyId}/jobs` | `ListJobs` | Optional `?status=` filter |
| `POST` | `/{agencyId}/jobs/{jobId}/cancel` | `CancelJob` | Transitions running→cancelled |

---

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
│   ├── sandbox/              — Linux namespace subprocess launcher
│   └── service/              — gRPC handler implementations
├── proto/
│   └── codevaldfunction/v1/
│       └── service.proto     — FunctionsService definition
├── gen/                      — generated gRPC/protobuf code
└── storage/                  — entitygraph adapter (collection names, graph name)
```
