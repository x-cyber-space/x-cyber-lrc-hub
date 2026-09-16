package scoring

import (
	"testing"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

func TestNormalizeIdentity(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		// The four behaviours measured against lrclib.net's /api/get.
		{"exact", "Wildest Dreams", "wildest dreams"},
		{"lowercase", "wildest dreams", "wildest dreams"},
		{"collapsed whitespace", "Wildest  Dreams", "wildest dreams"},
		{"stripped bracket suffix", "Wildest Dreams (Taylor's Version)", "wildest dreams"},
		{"no fuzzy folding", "Wildest Dream", "wildest dream"},
		{"chinese brackets", "七里香（伴奏）", "七里香"},
		{"full-width brackets", "七里香【超清】", "七里香"},
	}

	for _, c := range cases {
		if got := NormalizeIdentity(c.input); got != c.want {
			t.Errorf("%s: NormalizeIdentity(%q) = %q, want %q", c.name, c.input, got, c.want)
		}
	}
}

// TestNormalizeIdentityKeepsPunctuation guards the separation from
// NormalizeText: identity matching must not silently fold punctuation away.
func TestNormalizeIdentityKeepsPunctuation(t *testing.T) {
	if NormalizeIdentity("AC/DC") == NormalizeIdentity("ACDC") {
		t.Fatal("identity normalization must not erase '/' — AC/DC and ACDC are different artists")
	}
}

// TestMatchTrack mirrors the behaviour measured on the live lrclib.net
// /api/get endpoint.
func TestMatchTrack(t *testing.T) {
	base := model.Query{TrackName: "Wildest Dreams", ArtistName: "Taylor Swift", AlbumName: "1989", Duration: 220}

	cases := []struct {
		name    string
		query   model.Query
		track   string
		artist  string
		album   string
		dur     float64
		want    MatchStage
		comment string
	}{
		{
			name: "identical", query: base,
			track: "Wildest Dreams", artist: "Taylor Swift", album: "1989", dur: 220,
			want: MatchExact,
		},
		{
			name: "duration within +3s window", query: base,
			track: "Wildest Dreams", artist: "Taylor Swift", album: "1989", dur: 222,
			want: MatchExact, comment: "lrclib returns the 220s record for a 222s request",
		},
		{
			name: "duration outside window", query: base,
			track: "Wildest Dreams", artist: "Taylor Swift", album: "1989", dur: 230,
			want: MatchNone, comment: "lrclib 404s a 230s request",
		},
		{
			name: "bracket suffix is stripped", query: base,
			track: "Wildest Dreams (Taylor's Version)", artist: "Taylor Swift", album: "1989", dur: 220,
			want: MatchExact, comment: "lrclib matches the original record",
		},
		{
			name: "typo is not fuzzy-matched", query: base,
			track: "Wildest Dream", artist: "Taylor Swift", album: "1989", dur: 220,
			want: MatchNone, comment: "lrclib 404s a one-letter typo",
		},
		{
			// The exact stage enforces the album the way lrclib.net does, but a
			// mismatch there falls through to the relaxed stage rather than
			// 404ing: platform album strings are far dirtier than LRCLIB's own
			// curated column, and enforcing album outright turned songs that
			// plainly exist into misses.
			name: "wrong album falls through to the relaxed stage", query: base,
			track: "Wildest Dreams", artist: "Taylor Swift", album: "Wrong Album", dur: 220,
			want: MatchRelaxed,
		},
		{
			name: "album matches exactly when it does line up", query: base,
			track: "Wildest Dreams", artist: "Taylor Swift", album: "1989", dur: 220,
			want: MatchExact,
		},
		{
			name:  "album omitted imposes nothing",
			query: model.Query{TrackName: "Wildest Dreams", ArtistName: "Taylor Swift"},
			track: "Wildest Dreams", artist: "Taylor Swift", album: "1989", dur: 220,
			want: MatchExact,
		},
		{
			name:  "duration omitted imposes nothing",
			query: model.Query{TrackName: "Wildest Dreams", ArtistName: "Taylor Swift"},
			track: "Wildest Dreams", artist: "Taylor Swift", album: "", dur: 0,
			want: MatchExact,
		},

		// The regression this whole change exists for.
		{
			name:  "same title, unrelated artist, duration far off",
			query: model.Query{TrackName: "童话", ArtistName: "光良", Duration: 246},
			track: "童话", artist: "王菲", album: "", dur: 255,
			want: MatchNone, comment: "was 45.00 and passed the old >=40 threshold",
		},
		{
			name:  "same title, unrelated artist, no duration",
			query: model.Query{TrackName: "童话", ArtistName: "光良"},
			track: "童话", artist: "王菲", album: "", dur: 0,
			want: MatchNone, comment: "was 65.00 via the old free 20-point duration score",
		},
		{
			name:  "cover version, unrelated artist",
			query: model.Query{TrackName: "后来", ArtistName: "刘若英", Duration: 300},
			track: "后来", artist: "张敬轩", album: "", dur: 302,
			want: MatchNone, comment: "the app shows this as a grey candidate; a headless server must not commit to it",
		},

		// Relaxations that must keep working.
		{
			name:  "multi-artist credit overlaps",
			query: model.Query{TrackName: "千里之外", ArtistName: "周杰伦", Duration: 239},
			track: "千里之外", artist: "周杰伦 / 费玉清", album: "", dur: 239,
			want: MatchRelaxed,
		},
		{
			name:  "platform title suffix",
			query: model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269},
			track: "晴天 现场版", artist: "周杰伦", album: "", dur: 270,
			want: MatchRelaxed,
		},
		{
			name:  "relaxed duration window",
			query: model.Query{TrackName: "晴天", ArtistName: "周杰伦", Duration: 269},
			track: "晴天", artist: "周杰伦", album: "", dur: 273,
			want: MatchRelaxed, comment: "4s apart: outside ±3s, inside ±5s",
		},
		{
			name:  "unknown artist does not block",
			query: model.Query{TrackName: "晴天", ArtistName: "未知歌手", Duration: 269},
			track: "晴天", artist: "周杰伦", album: "", dur: 269,
			want: MatchExact,
		},
		{
			name:  "short title cannot swallow a longer one",
			query: model.Query{TrackName: "红", ArtistName: "陈奕迅", Duration: 260},
			track: "红玫瑰", artist: "陈奕迅", album: "", dur: 260,
			want: MatchNone, comment: "containment needs a request of at least two runes",
		},
	}

	for _, c := range cases {
		got := MatchTrack(c.query, c.track, c.artist, c.album, c.dur)
		if got != c.want {
			t.Errorf("%s: MatchTrack = %v, want %v (%s)", c.name, got, c.want, c.comment)
		}
	}
}

func TestArtistsOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"周杰伦", "周杰伦 / 费玉清", true},
		{"周杰伦 / 费玉清", "周杰伦", true},
		{"Beyond feat. 黄家驹", "Beyond", true},
		{"周杰伦", "王菲", false},
		{"AC/DC", "AC/DC", true},
		{"Simon & Garfunkel", "Simon & Garfunkel", true},
		{"", "周杰伦", false},
	}

	for _, c := range cases {
		if got := ArtistsOverlap(c.a, c.b); got != c.want {
			t.Errorf("ArtistsOverlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestDurationWithin(t *testing.T) {
	if !DurationWithin(0, 200, DurationWindowSeconds) {
		t.Error("missing target duration must impose no constraint")
	}
	if !DurationWithin(200, 0, DurationWindowSeconds) {
		t.Error("missing candidate duration must impose no constraint")
	}
	if !DurationWithin(220, 222, DurationWindowSeconds) {
		t.Error("2s apart must be inside the ±3s window")
	}
	if DurationWithin(220, 224, DurationWindowSeconds) {
		t.Error("4s apart must be outside the ±3s window")
	}
}

// TestCalculateScoreContract pins the fuzzy scorer's role: it ranks, it does
// not gate. A perfect title with an unrelated artist still scores highly here,
// which is precisely why /api/get must not use it as an acceptance threshold.
func TestCalculateScoreContract(t *testing.T) {
	score := CalculateScore("童话", "光良", 246, "童话", "王菲", 255)
	if score < 40 {
		t.Fatalf("expected the additive scorer to rate a title collision highly, got %.2f", score)
	}
	if MatchTrack(
		model.Query{TrackName: "童话", ArtistName: "光良", Duration: 246},
		"童话", "王菲", "", 255,
	) != MatchNone {
		t.Fatal("the matcher must reject what the scorer rates highly")
	}
}
