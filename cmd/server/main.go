// Command server starts the CodeValdFunctions gRPC microservice.
//
// Configuration is via environment variables — see internal/config for the
// full list.
package main

import (
	"log"

	"github.com/aosanya/CodeValdFunctions/internal/app"
	"github.com/aosanya/CodeValdFunctions/internal/config"
)

func main() {
	if err := app.Run(config.Load()); err != nil {
		log.Fatalf("codevaldelfunctions: %v", err)
	}
}
