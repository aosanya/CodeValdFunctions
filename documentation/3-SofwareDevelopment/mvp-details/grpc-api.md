# Job gRPC API — MVP-FN-006

## Overview

CodeValdFunctions exposes a gRPC API so other platform services and the UI can
query job history and manage running jobs.

---

## FunctionsService

```protobuf
service FunctionsService {
  // ListJobs returns jobs for this agency, optionally filtered.
  rpc ListJobs (ListJobsRequest) returns (ListJobsResponse);

  // GetJob returns a single job by ID.
  rpc GetJob (GetJobRequest) returns (Job);

  // CancelJob cancels a pending or running job.
  // Error: NOT_FOUND if the job does not exist.
  // Error: FAILED_PRECONDITION if the job is already in a terminal state.
  rpc CancelJob (CancelJobRequest) returns (CancelJobResponse);
}
```

### Messages

```protobuf
message Job {
  string id               = 1;
  string status           = 2;
  string function_name    = 3;
  string trigger_event    = 4;
  string trigger_payload  = 5;
  string result           = 6;
  string error            = 7;
  int32  retry_count      = 8;
  google.protobuf.Timestamp created_at   = 9;
  google.protobuf.Timestamp started_at   = 10;
  google.protobuf.Timestamp completed_at = 11;
}

message ListJobsRequest {
  string status        = 1; // optional filter by status
  string function_name = 2; // optional filter by function
  int32  limit         = 3;
}
message ListJobsResponse {
  repeated Job jobs = 1;
}

message GetJobRequest  { string job_id = 1; }
message CancelJobRequest  { string job_id = 1; }
message CancelJobResponse {}
```

---

## MVP-FN-006 Acceptance Criteria

- [ ] `ListJobs` returns correct jobs with optional status/function filters
- [ ] `GetJob` returns `NOT_FOUND` for unknown IDs
- [ ] `CancelJob` transitions job to `cancelled` from `pending` or `running`
- [ ] `CancelJob` returns `FAILED_PRECONDITION` for terminal-state jobs
