// Package config loads CodeValdFunctions runtime configuration from environment variables.
package config

import (
	"time"

	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Config holds all runtime configuration for the CodeValdFunctions service.
type Config struct {
	// GRPCPort is the port the gRPC server listens on.
	GRPCPort string

	// ArangoEndpoint is the ArangoDB HTTP endpoint (default "http://localhost:8529").
	ArangoEndpoint string

	// ArangoUser is the ArangoDB username (default "root").
	ArangoUser string

	// ArangoPassword is the ArangoDB password.
	ArangoPassword string

	// ArangoDatabase is the ArangoDB database name (default "codevaldfunctions").
	ArangoDatabase string

	// CrossGRPCAddr is the CodeValdCross gRPC address for registration heartbeats.
	// Empty string disables registration.
	CrossGRPCAddr string

	// AdvertiseAddr is the address CodeValdCross dials back on (default ":GRPCPort").
	AdvertiseAddr string

	// AgencyID is the agency ID sent in every Register heartbeat.
	AgencyID string

	// PingInterval is the heartbeat cadence (default 20s).
	PingInterval time.Duration

	// PingTimeout is the per-RPC timeout for each Register call (default 5s).
	PingTimeout time.Duration

	// GitCloneBase is the base URL for git cloning through Cross
	// (env: FN_GIT_CLONE_BASE, default "").
	GitCloneBase string

	// DefaultRepo is the default repo name to compile
	// (env: FN_DEFAULT_REPO, default "").
	DefaultRepo string

	// FunctionsDir is the directory scanned for function binaries and manifests.
	// (env: FN_FUNCTIONS_DIR, default "/opt/functions")
	FunctionsDir string
}

// Load reads configuration from environment variables, falling back to defaults.
//
// Environment variables:
//
//	CODEVALDFUNCTIONS_GRPC_PORT   gRPC listener port (required)
//	FN_ARANGO_ENDPOINT              ArangoDB endpoint (default "http://localhost:8529")
//	FN_ARANGO_USER                  ArangoDB username (default "root")
//	FN_ARANGO_PASSWORD              ArangoDB password
//	FN_ARANGO_DATABASE              ArangoDB database (default "codevaldfunctions")
//	CROSS_GRPC_ADDR                 CodeValdCross gRPC address (default ""; disables registration)
//	FN_GRPC_ADVERTISE_ADDR          address Cross dials back on (default ":PORT")
//	CODEVALDFUNCTIONS_AGENCY_ID   agency scope for this instance
//	CROSS_PING_INTERVAL             heartbeat cadence, Go duration string (default "20s")
//	CROSS_PING_TIMEOUT              per-RPC Register timeout (default "5s")
func Load() Config {
	port := serverutil.MustGetEnv("CODEVALDFUNCTIONS_GRPC_PORT")
	return Config{
		GRPCPort:       port,
		ArangoEndpoint: serverutil.EnvOrDefault("FN_ARANGO_ENDPOINT", "http://localhost:8529"),
		ArangoUser:     serverutil.EnvOrDefault("FN_ARANGO_USER", "root"),
		ArangoPassword: serverutil.EnvOrDefault("FN_ARANGO_PASSWORD", ""),
		ArangoDatabase: serverutil.EnvOrDefault("FN_ARANGO_DATABASE", "codevaldfunctions"),
		CrossGRPCAddr:  serverutil.EnvOrDefault("CROSS_GRPC_ADDR", ""),
		AdvertiseAddr:  serverutil.EnvOrDefault("FN_GRPC_ADVERTISE_ADDR", ":"+port),
		AgencyID:       serverutil.EnvOrDefault("CODEVALDFUNCTIONS_AGENCY_ID", ""),
		PingInterval:   serverutil.ParseDurationString("CROSS_PING_INTERVAL", 20*time.Second),
		PingTimeout:    serverutil.ParseDurationString("CROSS_PING_TIMEOUT", 5*time.Second),
		GitCloneBase:   serverutil.EnvOrDefault("FN_GIT_CLONE_BASE", ""),
		DefaultRepo:    serverutil.EnvOrDefault("FN_DEFAULT_REPO", ""),
		FunctionsDir:   serverutil.EnvOrDefault("FN_FUNCTIONS_DIR", "/opt/functions"),
	}
}
