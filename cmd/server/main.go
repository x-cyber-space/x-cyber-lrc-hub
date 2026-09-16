package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/api"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/config"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

const banner = `
__  __   ____ y b e r   _      ____    ____     _   _       _     
\ \/ /  / ___|         | |    |  _ \  / ___|   | | | | _   | |__  
 \  /  | |      _____  | |    | |_) || |       | |_| || |  | '_ \ 
 /  \  | |___  |_____| | |___ |  _ < | |___    |  _  || |_ | |_) |
/_/\_\  \____|         |_____||_| \_\ \____|   |_| |_| \___||_.__/ 
             :: X-Cyber Lyrics Hub :: [v1.0.0]
             :: LRCLIB Compatible API Server ::
`

func main() {
	fmt.Print(banner)

	cfg := config.ParseFlags()

	log.Printf("[init] starting X-Cyber Lyrics Hub on port :%d", cfg.Port)
	log.Printf("[init] cache path: %s", cfg.CachePath)
	log.Printf("[init] log level: %s", cfg.LogLevel)

	// Initialize SQLite Cache
	store, err := cache.NewSQLiteStore(cfg.CachePath)
	if err != nil {
		log.Fatalf("[fatal] failed to initialize cache store: %v", err)
	}
	defer store.Close()
	log.Printf("[init] local SQLite cache initialized")

	// Initialize Multi-Provider Dispatcher
	dispatcher := provider.NewDispatcher()
	log.Printf("[init] multi-source providers initialized (NetEase, QQ Music, Kugou, Kuwo)")

	// Initialize Router
	router := api.NewRouter(store, dispatcher)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[ready] HTTP server listening on http://0.0.0.0:%d", cfg.Port)
		log.Printf("[ready] LRCLIB endpoints available at /api/get and /api/search")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[fatal] HTTP server error: %v", err)
		}
	}()

	<-stopChan
	log.Println("[shutdown] shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[error] server forced to shutdown: %v", err)
	}

	log.Println("[shutdown] server exiting, goodbye!")
}
