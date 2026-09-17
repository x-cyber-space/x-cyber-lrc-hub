// Package matching decides whether a candidate is the same track a request
// asked for.
//
// This is a hard filter, not a ranking: identity is compared exactly (after
// normalisation) and a mismatched artist is a contradiction rather than a
// deduction. Ranking lives in the scoring package.

package matching

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
)

// MatchStage describes how confidently a candidate satisfied a query, in
// increasing order of confidence.
type MatchStage int

const (
	// MatchNone means the candidate is not a plausible match at all.
	MatchNone MatchStage = iota
	// MatchRelaxed means the candidate matched under controlled relaxation:
	// multi-artist credits ("周杰伦 / 费玉清"), a widened duration window, or a
	// platform title that carries an extra suffix.
	MatchRelaxed
	// MatchExact means the candidate matched under LRCLIB's own rules:
	// normalised equality on the supplied fields plus its duration window.
	MatchExact
)

// Duration windows used by the matcher.
//
// These mirror lrclib.net's /api/get, measured against the live service: a
// record stored at 220s is returned for requests of 220/221/222s, a request
// of 225s is redirected to the nearer 226s record, and a request of 230s
// misses entirely. So the tolerance is a ±3s window with nearest-match
// selection — not a scored contribution.
const (
	DurationWindowSeconds        = 3.0
	DurationWindowRelaxedSeconds = 5.0

	// minRelaxedTitleRunes is the shortest request title allowed to use
	// containment matching. Without it a request for "红" is swallowed by any
	// longer title that merely contains it, such as "红玫瑰" — a different
	// song. Exact equality is unaffected by this floor.
	minRelaxedTitleRunes = 2
)

// MatchTrack classifies a candidate against a query.
//
// Note what this deliberately does NOT do: it never accumulates a weighted
// score. An additive model lets a full-marks title (45) carry a completely
// unrelated artist (0) over the line, which is how a lyrics service ends up
// serving a cover version's lyrics against the original. Mismatched identity
// is a hard contradiction, not a deduction.
func MatchTrack(q model.Query, trackName, artistName, albumName string, duration float64) MatchStage {
	if matchExact(q, trackName, artistName, albumName, duration) {
		return MatchExact
	}
	if matchRelaxed(q, trackName, artistName, albumName, duration) {
		return MatchRelaxed
	}
	return MatchNone
}

// matchExact reproduces LRCLIB's /api/get filter: compare the normalised
// identity, and apply each optional field only when the request supplied it.
func matchExact(q model.Query, trackName, artistName, albumName string, duration float64) bool {
	want := NormalizeIdentity(q.TrackName)
	got := NormalizeIdentity(trackName)
	if want == "" || want != got {
		return false
	}

	if !ArtistUnknown(q.ArtistName) {
		if NormalizeIdentity(q.ArtistName) != NormalizeIdentity(artistName) {
			return false
		}
	}

	if q.AlbumName != "" && NormalizeIdentity(q.AlbumName) != NormalizeIdentity(albumName) {
		return false
	}

	return DurationWithin(q.Duration, duration, DurationWindowSeconds)
}

// matchRelaxed is the second stage, aimed at the messy metadata produced by
// Chinese streaming platforms while keeping every relaxation deterministic
// and free of any model call.
func matchRelaxed(q model.Query, trackName, artistName, albumName string, duration float64) bool {
	want := NormalizeIdentity(q.TrackName)
	got := NormalizeIdentity(trackName)
	if want == "" || got == "" {
		return false
	}

	// Platforms routinely append a suffix the request does not carry
	// ("晴天" vs "晴天 现场版"). Both guards matter: the candidate must be the
	// *longer* string, and the request must be long enough to be meaningful,
	// so a two-character title cannot swallow an unrelated longer one and a
	// one-character title cannot match anything by containment at all.
	if want != got {
		contained := len(got) > len(want) &&
			strings.Contains(got, want) &&
			utf8.RuneCountInString(want) >= minRelaxedTitleRunes
		if !contained {
			return false
		}
	}

	// Multi-artist credits: "周杰伦" must match "周杰伦 / 费玉清".
	if !ArtistUnknown(q.ArtistName) && !ArtistsOverlap(q.ArtistName, artistName) {
		return false
	}

	// Album is deliberately NOT enforced here.
	//
	// lrclib.net treats album as a hard filter, but its album column is its
	// own curated data. Ours comes from streaming platforms whose album
	// strings disagree constantly with a listener's file tags (edition
	// suffixes, traditional/simplified variants, compilation re-issues), and
	// enforcing it here turned songs that plainly exist into 404s. The exact
	// stage still enforces it, so a correctly tagged client sees LRCLIB's
	// behaviour verbatim; this stage only rescues the messy case. Album also
	// carries almost no information about the *lyrics*, which are the same
	// across the album that a track appears on.
	return DurationWithin(q.Duration, duration, DurationWindowRelaxedSeconds)
}

// DurationWithin reports whether a candidate duration sits inside the window.
//
// An unknown duration on either side imposes no constraint. This matches
// LRCLIB, which resolves a request carrying no `duration` purely on name and
// artist, and it keeps a missing duration from silently becoming free points
// the way a "neutral score" would.
func DurationWithin(target, candidate, window float64) bool {
	if target <= 0 || candidate <= 0 {
		return true
	}
	return math.Abs(target-candidate) <= window
}

// DurationDistance orders candidates by proximity to the requested duration.
// Unknown durations sort last so they never win a tie-break.
func DurationDistance(target, candidate float64) float64 {
	if target <= 0 || candidate <= 0 {
		return math.MaxFloat64
	}
	return math.Abs(target - candidate)
}

// ArtistUnknown reports whether a credit string carries no usable artist
// name, in which case artist agreement cannot be required.
func ArtistUnknown(artist string) bool {
	return IsUnknownArtist(artist)
}
