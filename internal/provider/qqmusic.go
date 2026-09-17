package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/lyrics"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// QQMusicProvider implements lyrics search for QQ Music
type QQMusicProvider struct{}

func NewQQMusicProvider() *QQMusicProvider {
	return &QQMusicProvider{}
}

func (p *QQMusicProvider) Name() string {
	return "qqmusic"
}

type qqSearchResp struct {
	Code int `json:"code"`
	Data struct {
		Song struct {
			List []struct {
				Songmid   string `json:"songmid"`
				Songname  string `json:"songname"`
				Albumname string `json:"albumname"`
				Interval  int    `json:"interval"` // seconds
				Singer    []struct {
					Name string `json:"name"`
				} `json:"singer"`
			} `json:"list"`
		} `json:"song"`
	} `json:"data"`
}

type qqLyricResp struct {
	Retcode int    `json:"retcode"`
	Code    int    `json:"code"`
	Lyric   string `json:"lyric"`
}

func (p *QQMusicProvider) Search(ctx context.Context, keyword string, limit int) ([]*model.Candidate, error) {
	if limit <= 0 {
		limit = 5
	}

	searchURL := fmt.Sprintf("https://c.y.qq.com/soso/fcgi-bin/client_search_cp?p=1&n=%d&w=%s&format=json",
		limit, url.QueryEscape(keyword))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Referer", "https://y.qq.com")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data qqSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var candidates []*model.Candidate
	for _, s := range data.Data.Song.List {
		var singerNames []string
		for _, singer := range s.Singer {
			singerNames = append(singerNames, singer.Name)
		}

		cand := &model.Candidate{
			Source:     p.Name(),
			SongID:     s.Songmid,
			TrackName:  s.Songname,
			ArtistName: strings.Join(singerNames, " / "),
			AlbumName:  s.Albumname,
			Duration:   float64(s.Interval),
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *QQMusicProvider) FetchLyrics(ctx context.Context, candidate *model.Candidate) error {
	lyricURL := fmt.Sprintf("https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=%s&format=json&nobase64=1",
		candidate.SongID)

	req, err := http.NewRequestWithContext(ctx, "GET", lyricURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Referer", "https://y.qq.com")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data qqLyricResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	rawLyric := data.Lyric
	// In case QQ music returns base64 string
	if !strings.Contains(rawLyric, "[") && len(rawLyric) > 20 {
		if decoded, err := base64.StdEncoding.DecodeString(rawLyric); err == nil {
			rawLyric = string(decoded)
		}
	}

	// Unescape HTML entities, e.g. &#58; -> : and &#10; -> \n
	rawLyric = html.UnescapeString(rawLyric)
	rawLyric = strings.ReplaceAll(rawLyric, "\r\n", "\n")
	synced := strings.TrimSpace(rawLyric)

	plain := lyrics.ExtractPlainLyrics(synced)
	candidate.SyncedLyrics = synced
	candidate.PlainLyrics = plain
	candidate.Instrumental = lyrics.IsInstrumental(plain, synced)

	return nil
}
