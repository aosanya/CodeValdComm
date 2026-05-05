// Command server is the production CodeValdComm gRPC microservice. Configuration
// is read strictly from environment variables (see internal/config for the full
// list). No .env is loaded; the container/orchestrator is expected to inject
// the environment.
package main

import (
	"log"

	"github.com/aosanya/CodeValdComm/internal/app"
	"github.com/aosanya/CodeValdComm/internal/config"
)

func main() {
	if err := app.Run(config.Load()); err != nil {
		log.Fatalf("codevaldcomm: %v", err)
	}
}
