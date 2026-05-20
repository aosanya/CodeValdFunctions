// Package app holds the shared runtime wiring for CodeValdFunctions.
package app

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	codevaldelfunctions "github.com/aosanya/CodeValdFunctions"
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldelfunctions/v1"
	"github.com/aosanya/CodeValdFunctions/internal/config"
	"github.com/aosanya/CodeValdFunctions/internal/registrar"
	"github.com/aosanya/CodeValdFunctions/internal/server"
	fnarangodb "github.com/aosanya/CodeValdFunctions/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	healthpb "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldhealth/v1"
	"github.com/aosanya/CodeValdSharedLib/health"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Run starts the CodeValdFunctions service and blocks until SIGINT/SIGTERM.
func Run(cfg config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Cross registrar (optional) ───────────────────────────────────────────
	var pub codevaldelfunctions.CrossPublisher
	if cfg.CrossGRPCAddr != "" {
		reg, err := registrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			cfg.AgencyID,
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if err != nil {
			log.Printf("codevaldelfunctions: registrar: failed to create: %v — continuing without registration", err)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
			pub = reg
		}
	} else {
		log.Println("codevaldelfunctions: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

	// ── ArangoDB backend ─────────────────────────────────────────────────────
	backend, err := fnarangodb.NewBackend(fnarangodb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
		Schema:   codevaldelfunctions.DefaultFunctionsSchema(),
	})
	if err != nil {
		return err
	}

	// ── Schema seed (idempotent on startup) ──────────────────────────────────
	if cfg.AgencyID != "" {
		seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := entitygraph.SeedSchema(seedCtx, backend, cfg.AgencyID, codevaldelfunctions.DefaultFunctionsSchema()); err != nil {
			log.Printf("codevaldelfunctions: schema seed: %v", err)
		}
		seedCancel()
	} else {
		log.Println("codevaldelfunctions: CODEVALDELFUNCTIONS_AGENCY_ID not set — skipping schema seed")
	}

	mgr := codevaldelfunctions.NewFunctionsManager(backend, pub, cfg.AgencyID)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterFunctionsServiceServer(grpcServer, server.New(mgr))
	healthpb.RegisterHealthServiceServer(grpcServer, health.New("codevaldelfunctions"))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("codevaldelfunctions: shutdown signal received")
		cancel()
	}()

	log.Printf("codevaldelfunctions: gRPC server listening on :%s", cfg.GRPCPort)
	serverutil.RunWithGracefulShutdown(ctx, grpcServer, lis, 30*time.Second)
	return nil
}
