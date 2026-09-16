package config

import (
	"flag"
	"fmt"
	"time"
)

// DefaultCacheTTL is how long a cached lyric is served before it is refetched.
//
// Lyrics do change upstream (a provider corrects a typo, adds a translation or
// replaces a badly timed LRC), and without an expiry the copy fetched on the
// first request would be served forever.
const DefaultCacheTTL = 30 * 24 * time.Hour

// Config represents runtime configuration.
type Config struct {
	Port      int
	CachePath string
	CacheTTL  time.Duration
	LogLevel  string
}

// ParseFlags parses command line arguments into Config.
//
// The returned error covers values that are structurally unusable; the log
// level is validated by logging.Setup so that this package stays independent
// of the logging implementation.
func ParseFlags() (*Config, error) {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", 3300, "Server listening port")
	flag.StringVar(&cfg.CachePath, "cache", "./data/lyrics.db", "SQLite cache file path")
	flag.DurationVar(&cfg.CacheTTL, "cache-ttl", DefaultCacheTTL,
		"How long a cached lyric stays fresh, e.g. 720h; 0 keeps cached lyrics forever")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port %d is out of range (1-65535)", cfg.Port)
	}
	if cfg.CachePath == "" {
		return nil, fmt.Errorf("cache path must not be empty")
	}
	if cfg.CacheTTL < 0 {
		return nil, fmt.Errorf("cache ttl %s must not be negative", cfg.CacheTTL)
	}

	return cfg, nil
}
