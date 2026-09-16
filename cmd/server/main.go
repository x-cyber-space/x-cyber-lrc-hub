package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/api"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/config"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/logging"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// version is the single source of truth for the reported version. Override it
// at build time rather than editing this file:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always)" ./cmd/server
var version = "1.0.0"

const bannerArt = `
__  __   ____ y b e r   _      ____    ____     _   _       _     
\ \/ /  / ___|         | |    |  _ \  / ___|   | | | | _   | |__  
 \  /  | |      _____  | |    | |_) || |       | |_| || |  | '_ \ 
 /  \  | |___  |_____| | |___ |  _ < | |___    |  _  || |_ | |_) |
/_/\_\  \____|         |_____||_| \_\ \____|   |_| |_| \___||_.__/ 
`

func banner() string {
	return fmt.Sprintf("%s             :: X-Cyber Lyrics Hub :: [v%s]\n"+
		"             :: LRCLIB Compatible API Server ::\n", bannerArt, version)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[fatal] %v\n", err)
		os.Exit(1)
	}
}

// run holds the real main so that every failure path can return an error and
// deferred cleanup still executes. The previous version called log.Fatalf from
// the middle of startup, which skipped every defer.
func run() error {
	cfg, err := config.ParseFlags()
	if err != nil {
		return err
	}
	if err := logging.Setup(cfg.LogLevel); err != nil {
		return err
	}

	fmt.Print(banner())

	slog.Info("starting x-cyber-lyrics-hub",
		"version", version,
		"port", cfg.Port,
		"cache", cfg.CachePath,
		"cacheTTL", cfg.CacheTTL.String(),
		"logLevel", cfg.LogLevel,
	)

	store, err := cache.NewSQLiteStore(cfg.CachePath, cfg.CacheTTL)
	if err != nil {
		return fmt.Errorf("failed to initialize cache store: %w", err)
	}
	defer store.Close()

	if removed, err := store.PruneExpired(); err != nil {
		slog.Warn("initial cache prune failed", "error", err)
	} else if removed > 0 {
		slog.Info("pruned expired cache rows", "rows", removed)
	}

	pruneCtx, stopPruner := context.WithCancel(context.Background())
	defer stopPruner()
	go pruneLoop(pruneCtx, store)

	dispatcher := provider.NewDispatcher()
	slog.Info("multi-source providers initialized",
		"providers", []string{"netease", "qqmusic", "kugou", "kuwo"})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: api.NewRouter(store, dispatcher, version),
		// ReadHeaderTimeout is what actually bounds a slowloris connection;
		// ReadTimeout alone still lets a client dribble headers indefinitely
		// within the window.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown channel.
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("HTTP server listening", "addr", "http://0.0.0.0:"+fmt.Sprint(cfg.Port))
		slog.Info("LRCLIB endpoints available", "endpoints", []string{"/api/get", "/api/get/{id}", "/api/search"})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case sig := <-stopChan:
		slog.Info("shutting down gracefully", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	slog.Info("server exiting, goodbye")
	return nil
}

// pruneLoop reclaims disk from expired rows. Reads already treat an expired
// row as a miss, so this is housekeeping rather than correctness; it runs at
// most hourly and never more often than a minute.
func pruneLoop(ctx context.Context, store *cache.SQLiteStore) {
	ttl := store.TTL()
	if ttl <= 0 {
		slog.Info("cache expiry disabled; cached lyrics are kept indefinitely")
		return
	}

	interval := ttl / 4
	if interval > time.Hour {
		interval = time.Hour
	}
	if interval < time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := store.PruneExpired()
			if err != nil {
				slog.Error("cache prune failed", "error", err)
				continue
			}
			if removed > 0 {
				slog.Info("pruned expired cache rows", "rows", removed)
			}
		}
	}
}
