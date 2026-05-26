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

	codevaldfunctions "github.com/aosanya/CodeValdFunctions"
	pb "github.com/aosanya/CodeValdFunctions/gen/go/codevaldfunctions/v1"
	"github.com/aosanya/CodeValdFunctions/internal/config"
	"github.com/aosanya/CodeValdFunctions/internal/functions"
	"github.com/aosanya/CodeValdFunctions/internal/registrar"
	"github.com/aosanya/CodeValdFunctions/internal/server"
	fnarangodb "github.com/aosanya/CodeValdFunctions/storage/arangodb"
	"github.com/aosanya/CodeValdSharedLib/entitygraph"
	healthpb "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldhealth/v1"
	sharedev1 "github.com/aosanya/CodeValdSharedLib/gen/go/codevaldshared/v1"
	"github.com/aosanya/CodeValdSharedLib/health"
	"github.com/aosanya/CodeValdSharedLib/serverutil"
)

// Run starts the CodeValdFunctions service and blocks until SIGINT/SIGTERM.
func Run(cfg config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Cross registrar (optional) ───────────────────────────────────────────
	var pub codevaldfunctions.CrossPublisher
	if cfg.CrossGRPCAddr != "" {
		reg, err := registrar.New(
			cfg.CrossGRPCAddr,
			cfg.AdvertiseAddr,
			cfg.AgencyID,
			cfg.PingInterval,
			cfg.PingTimeout,
		)
		if err != nil {
			log.Printf("codevaldfunctions: registrar: failed to create: %v — continuing without registration", err)
		} else {
			defer reg.Close()
			go reg.Run(ctx)
			pub = reg
		}
	} else {
		log.Println("codevaldfunctions: CROSS_GRPC_ADDR not set — skipping CodeValdCross registration")
	}

	// ── ArangoDB backend ─────────────────────────────────────────────────────
	backend, err := fnarangodb.NewBackend(fnarangodb.Config{
		Endpoint: cfg.ArangoEndpoint,
		Username: cfg.ArangoUser,
		Password: cfg.ArangoPassword,
		Database: cfg.ArangoDatabase,
		Schema:   codevaldfunctions.DefaultFunctionsSchema(),
	})
	if err != nil {
		return err
	}

	// ── Schema seed (idempotent on startup) ──────────────────────────────────
	if cfg.AgencyID != "" {
		seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := entitygraph.SeedSchema(seedCtx, backend, cfg.AgencyID, codevaldfunctions.DefaultFunctionsSchema()); err != nil {
			log.Printf("codevaldfunctions: schema seed: %v", err)
		}
		seedCancel()
	} else {
		log.Println("codevaldfunctions: CODEVALDFUNCTIONS_AGENCY_ID not set — skipping schema seed")
	}

	mgr := codevaldfunctions.NewFunctionsManager(backend, pub, cfg.AgencyID)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	runner := functions.NewBinaryRunner(cfg.FunctionsDir)
	log.Printf("codevaldfunctions: function runner scanning %s", cfg.FunctionsDir)

	grpcServer, _ := serverutil.NewGRPCServer()
	pb.RegisterFunctionsServiceServer(grpcServer, server.New(mgr, runner))
	healthpb.RegisterHealthServiceServer(grpcServer, health.New("codevaldfunctions"))
	if cfg.AgencyID != "" {
		dispatcher := server.NewFunctionsDispatcher(mgr, runner, cfg.AgencyID)
		sharedev1.RegisterEventReceiverServiceServer(grpcServer, server.NewEventReceiver(backend, cfg.AgencyID, dispatcher))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("codevaldfunctions: shutdown signal received")
		cancel()
	}()

	log.Printf("codevaldfunctions: gRPC server listening on :%s", cfg.GRPCPort)
	serverutil.RunWithGracefulShutdown(ctx, grpcServer, lis, 30*time.Second)
	return nil
}
