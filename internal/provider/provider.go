package provider

import (
	"context"
	"net/http"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// Provider defines the interface for music lyric sources
type Provider interface {
	Name() string
	Search(ctx context.Context, keyword string, limit int) ([]*model.Candidate, error)
	FetchLyrics(ctx context.Context, candidate *model.Candidate) error
}

// defaultHTTPClient provides a shared client with standard timeouts
var defaultHTTPClient = &http.Client{
	Timeout: 8 * time.Second,
}

// Common headers
const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
)
