package config

import (
	"flag"
)

// Config represents runtime configuration
type Config struct {
	Port      int
	CachePath string
	LogLevel  string
}

// ParseFlags parses command line arguments into Config
func ParseFlags() *Config {
	cfg := &Config{}
	flag.IntVar(&cfg.Port, "port", 3300, "Server listening port")
	flag.StringVar(&cfg.CachePath, "cache", "./data/lyrics.db", "SQLite cache file path")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()
	return cfg
}
