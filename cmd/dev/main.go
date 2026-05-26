// Command dev runs CodeValdFunctions locally. Defaults differ from production:
// listens on :50062, talks to http://localhost:8529, and leaves CROSS_GRPC_ADDR
// empty so dev runs standalone (no Cross required).
// The Makefile's `make dev` target sources CodeValdFunctions/.env before exec.
package main

import (
	"log"
	"os"

	"github.com/aosanya/CodeValdFunctions/internal/app"
	"github.com/aosanya/CodeValdFunctions/internal/config"
)

func main() {
	setDefault("CODEVALDFUNCTIONS_GRPC_PORT", "50062")
	setDefault("FN_ARANGO_ENDPOINT", "http://localhost:8529")

	log.Println("codevaldfunctions[dev]: starting with local-dev defaults")
	if err := app.Run(config.Load()); err != nil {
		log.Fatalf("codevaldfunctions[dev]: %v", err)
	}
}

func setDefault(key, val string) {
	if _, ok := os.LookupEnv(key); !ok {
		os.Setenv(key, val)
	}
}
