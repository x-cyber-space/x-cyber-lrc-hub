package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/util"
)

// KuwoProvider implements lyrics search for Kuwo Music
type KuwoProvider struct{}

func NewKuwoProvider() *KuwoProvider {
	return &KuwoProvider{}
}

func (p *KuwoProvider) Name() string {
	return "kuwo"
}

type kuwoSearchResp struct {
	Abslist []struct {
		Musicrid string      `json:"MUSICRID"`
		Songname string      `json:"SONGNAME"`
		Artist   string      `json:"ARTIST"`
		Album    string      `json:"ALBUM"`
		Duration interface{} `json:"DURATION"`
	} `json:"abslist"`
}

type kuwoLyricResp struct {
	Data struct {
		Lrclist []util.KuwoLrcItem `json:"lrclist"`
	} `json:"data"`
}

func (p *KuwoProvider) Search(ctx context.Context, keyword string, limit int) ([]*model.Candidate, error) {
	if limit <= 0 {
		limit = 5
	}

	searchURL := fmt.Sprintf("http://search.kuwo.cn/r.s?client=kt&all=%s&pn=0&rn=%d&uid=794764845&ver=kwplayer_ar_99.99.99.99&vipver=1&show_copyright_off=1&ft=music&cluster=0&strategy=2012&encoding=utf8&rformat=json&vermerge=1&mobi=1",
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

	var data kuwoSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var candidates []*model.Candidate
	for _, item := range data.Abslist {
		cleanID := strings.TrimPrefix(item.Musicrid, "MUSIC_")
		var dur float64
		switch v := item.Duration.(type) {
		case string:
			dur, _ = strconv.ParseFloat(v, 64)
		case float64:
			dur = v
		case int:
			dur = float64(v)
		}

		cand := &model.Candidate{
			Source:     p.Name(),
			SongID:     cleanID,
			TrackName:  item.Songname,
			ArtistName: item.Artist,
			AlbumName:  item.Album,
			Duration:   dur,
		}
		candidates = append(candidates, cand)
	}

	return candidates, nil
}

func (p *KuwoProvider) FetchLyrics(ctx context.Context, candidate *model.Candidate) error {
	lyricURL := fmt.Sprintf("http://m.kuwo.cn/newh5/singles/songinfoandlrc?musicId=%s&httpsStatus=1", candidate.SongID)

	req, err := http.NewRequestWithContext(ctx, "GET", lyricURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Referer", "http://m.kuwo.cn")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data kuwoLyricResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	if len(data.Data.Lrclist) == 0 {
		return fmt.Errorf("no lyrics found for kuwo id %s", candidate.SongID)
	}

	synced, plain := util.FormatKuwoLrc(data.Data.Lrclist)
	candidate.SyncedLyrics = synced
	candidate.PlainLyrics = plain
	candidate.Instrumental = util.IsInstrumental(plain, synced)

	return nil
}
