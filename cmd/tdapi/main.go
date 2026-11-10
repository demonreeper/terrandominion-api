// Command tdapi is the Terran Dominion game server.
//
//	tdapi serve         run the HTTP API (applies pending migrations first)
//	tdapi migrate       apply pending migrations and exit
//	tdapi job <name>    run one scheduled job and exit (node-regrow, account-cleanup, session-prune)
//	tdapi import-osm <file>  load the node seeder's map layers (scripts/import-osm.ps1 makes the file)
//	tdapi seed <area|all>    pre-seed the nodes of an area in td.seed_areas
//	tdapi version       print the build version
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/config"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/httpapi"
	"github.com/demonreeper/terrandominion/api/internal/jobs"
	"github.com/demonreeper/terrandominion/api/internal/service"
)

// version is set at build time: go build -ldflags "-X main.version=..."
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if os.Args[1] == "version" {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	setupLogging(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("E1900 database unavailable", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	switch os.Args[1] {
	case "migrate":
		if err := db.Migrate(ctx, conn); err != nil {
			slog.Error("E1900 migration failed", "err", err)
			os.Exit(1)
		}
	case "job":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "job name required:", jobs.Names())
			os.Exit(2)
		}
		if err := jobs.Run(ctx, conn, os.Args[2]); err != nil {
			slog.Error("E1900 job failed", "job", os.Args[2], "err", err)
			os.Exit(1)
		}
	case "import-osm":
		// The seeder's land, walkways and blocked areas, from deploy/maps/osm_extract.py.
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tdapi import-osm <extract.json>")
			os.Exit(2)
		}
		f, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		land, walk, blocked, err := service.New(conn, cfg).ImportOSM(ctx, f)
		if err != nil {
			slog.Error("E1900 OSM import failed", "err", err)
			os.Exit(1)
		}
		fmt.Printf("imported %d land, %d walkway and %d blocked shapes\n", land, walk, blocked)
	case "seed":
		// Pre-seeds one area from td.seed_areas, or every area with "all".
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tdapi seed <area id | all>")
			os.Exit(2)
		}
		svc := service.New(conn, cfg)
		ids := []string{os.Args[2]}
		if os.Args[2] == "all" {
			areas, err := svc.SeedAreaIDs(ctx)
			if err != nil {
				slog.Error("E1900 seed areas unavailable", "err", err)
				os.Exit(1)
			}
			ids = ids[:0]
			for _, a := range areas {
				ids = append(ids, a.ID)
			}
		}
		for _, id := range ids {
			res, err := svc.SeedArea(ctx, 0, id)
			if err != nil {
				slog.Error("E1900 seeding failed", "area", id, "err", err)
				os.Exit(1)
			}
			fmt.Printf("%s: %d cells, %d nodes placed\n", res.Area, res.Cells, res.Created)
		}
	case "serve":
		if err := db.Migrate(ctx, conn); err != nil {
			slog.Error("E1900 migration failed", "err", err)
			os.Exit(1)
		}
		devLogin, _ := strconv.ParseBool(os.Getenv("TD_DEV_LOGIN"))
		serve(ctx, cfg, service.New(conn, cfg), devLogin)
	default:
		usage()
		os.Exit(2)
	}
}

func serve(ctx context.Context, cfg config.Config, svc *service.Service, devLogin bool) {
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(svc, cfg, devLogin).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	go svc.RunTetherLoop(ctx)
	go svc.RunPushLoop(ctx)

	slog.Info("tdapi listening", "addr", cfg.Addr, "env", cfg.Env, "version", version, "devLogin", devLogin && !cfg.IsLive())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("E1900 server stopped", "err", err)
		os.Exit(1)
	}
	slog.Info("tdapi stopped")
}

// setupLogging writes JSON lines to stdout; systemd's journal keeps them. Dev logs every
// request, live only notable events.
func setupLogging(cfg config.Config) {
	level := slog.LevelInfo
	if !cfg.IsLive() {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tdapi serve | migrate | job <name> | import-osm <file> | seed <area|all> | version")
}
