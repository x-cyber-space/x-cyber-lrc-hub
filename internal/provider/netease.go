package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/lyrics"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// NetEaseProvider implements lyrics search for NetEase Cloud Music
type NetEaseProvider struct{}

func NewNetEaseProvider() *NetEaseProvider {
	return &NetEaseProvider{}
}

func (p *NetEaseProvider) Name() string {
	return "netease"
}

type netEaseSearchResp struct {
	Result struct {
		Songs []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Name string `json:"name"`
			} `json:"album"`
			Duration int64 `json:"duration"` // in milliseconds
		} `json:"songs"`
	} `json:"result"`
}

type netEaseLyricResp struct {
	Lrc struct {
		Lyric string `json:"lyric"`
	} `json:"lrc"`
	Nolyric     bool `json:"nolyric"`
	Uncollected bool `json:"uncollected"`
}

func (p *NetEaseProvider) Search(ctx context.Context, keyword string, limit int) ([]*model.Candidate, error) {
	if limit <= 0 {
		limit = 5
	}

	searchURL := fmt.Sprintf("https://music.163.com/api/search/get/web?csrf_token=&type=1&limit=%d&s=%s",
		limit, url.QueryEscape(keyword))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data netEaseSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var candidates []*model.Candidate
	for _, s := range data.Result.Songs {
		var artistNames []string
		for _, a := range s.Artists {
			artistNames = append(artistNames, a.Name)
		}

		cand := &model.Candidate{
			Source:     p.Name(),
			SongID:     strconv.FormatInt(s.ID, 10),
			TrackName:  s.Name,
			ArtistName: strings.Join(artistNames, " / "),
			AlbumName:  s.Album.Name,
			Duration:   float64(s.Duration) / 1000.0,
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *NetEaseProvider) FetchLyrics(ctx context.Context, candidate *model.Candidate) error {
	lyricURL := fmt.Sprintf("https://music.163.com/api/song/lyric?os=pc&id=%s&lv=-1&kv=-1&tv=-1", candidate.SongID)

	req, err := http.NewRequestWithContext(ctx, "GET", lyricURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Referer", "https://music.163.com")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data netEaseLyricResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	if data.Nolyric || data.Uncollected {
		candidate.Instrumental = true
		candidate.SyncedLyrics = ""
		candidate.PlainLyrics = ""
		return nil
	}

	synced := strings.TrimSpace(data.Lrc.Lyric)
	plain := lyrics.ExtractPlainLyrics(synced)
	candidate.SyncedLyrics = synced
	candidate.PlainLyrics = plain
	candidate.Instrumental = lyrics.IsInstrumental(plain, synced)

	return nil
}
