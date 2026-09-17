package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/lyrics"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// KugouProvider implements lyrics search for Kugou Music
type KugouProvider struct{}

func NewKugouProvider() *KugouProvider {
	return &KugouProvider{}
}

func (p *KugouProvider) Name() string {
	return "kugou"
}

type kugouSearchResp struct {
	Status int `json:"status"`
	Data   struct {
		Info []struct {
			Hash       string `json:"hash"`
			Songname   string `json:"songname"`
			Singername string `json:"singername"`
			AlbumName  string `json:"album_name"`
			Duration   int    `json:"duration"` // seconds
		} `json:"info"`
	} `json:"data"`
}

type kugouLrcSearchResp struct {
	Candidates []struct {
		ID        string `json:"id"`
		Accesskey string `json:"accesskey"`
	} `json:"candidates"`
}

type kugouLrcDownloadResp struct {
	Status  int    `json:"status"`
	Content string `json:"content"` // base64 encoded
}

func (p *KugouProvider) Search(ctx context.Context, keyword string, limit int) ([]*model.Candidate, error) {
	if limit <= 0 {
		limit = 5
	}

	searchURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/search/song?format=json&keyword=%s&page=1&pagesize=%d",
		url.QueryEscape(keyword), limit)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data kugouSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var candidates []*model.Candidate
	for _, item := range data.Data.Info {
		cand := &model.Candidate{
			Source:     p.Name(),
			SongID:     item.Hash,
			TrackName:  item.Songname,
			ArtistName: item.Singername,
			AlbumName:  item.AlbumName,
			Duration:   float64(item.Duration),
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *KugouProvider) FetchLyrics(ctx context.Context, candidate *model.Candidate) error {
	// Step 1: Search for lyric id and accesskey using file hash
	searchLrcURL := fmt.Sprintf("http://krcs.kugou.com/search?ver=1&man=yes&client=mobi&keyword=&duration=&hash=%s",
		candidate.SongID)

	req, err := http.NewRequestWithContext(ctx, "GET", searchLrcURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var searchData kugouLrcSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return err
	}

	if len(searchData.Candidates) == 0 {
		return fmt.Errorf("no lyrics candidate found in kugou for hash %s", candidate.SongID)
	}

	best := searchData.Candidates[0]

	// Step 2: Download the lyric
	downloadURL := fmt.Sprintf("http://krcs.kugou.com/download?ver=1&client=mobi&fmt=lrc&charset=utf8&id=%s&accesskey=%s",
		best.ID, best.Accesskey)

	req2, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req2.Header.Set("User-Agent", defaultUserAgent)

	resp2, err := defaultHTTPClient.Do(req2)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	var dlData kugouLrcDownloadResp
	if err := json.NewDecoder(resp2.Body).Decode(&dlData); err != nil {
		return err
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(dlData.Content)
	if err != nil {
		return fmt.Errorf("failed to decode base64 lyrics from kugou: %w", err)
	}

	synced := strings.ReplaceAll(string(decodedBytes), "\r\n", "\n")
	synced = strings.TrimSpace(synced)
	plain := lyrics.ExtractPlainLyrics(synced)

	candidate.SyncedLyrics = synced
	candidate.PlainLyrics = plain
	candidate.Instrumental = lyrics.IsInstrumental(plain, synced)

	return nil
}
