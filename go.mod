module github.com/aosanya/CodeValdFunctions

go 1.25.3

require (
	github.com/aosanya/CodeValdSharedLib v0.0.0
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11
)

replace github.com/aosanya/CodeValdSharedLib => ../CodeValdSharedLib

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)
