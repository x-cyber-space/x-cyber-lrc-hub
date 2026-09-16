package util

import (
	"strings"
	"testing"
)

func TestExtractPlainLyrics(t *testing.T) {
	synced := `[ti:七里香]
[ar:周杰伦]
[al:七里香]
[00:00.00] 作词 : 方文山
[00:02.00] 作曲 : 周杰伦
[00:18.50]窗外的麻雀 在电线杆上多嘴
[00:22.80]你说这一句 很有夏天的感觉`

	plain := ExtractPlainLyrics(synced)
	expected := "作词 : 方文山\n作曲 : 周杰伦\n窗外的麻雀 在电线杆上多嘴\n你说这一句 很有夏天的感觉"
	if plain != expected {
		t.Fatalf("Expected:\n%s\nGot:\n%s", expected, plain)
	}
}

func TestIsInstrumental(t *testing.T) {
	if !IsInstrumental("纯音乐，请欣赏", "[00:00.00] 纯音乐，请欣赏") {
		t.Errorf("Expected true for instrumental")
	}

	if IsInstrumental("窗外的麻雀 在电线杆上多嘴", "[00:18.50]窗外的麻雀 在电线杆上多嘴") {
		t.Errorf("Expected false for non-instrumental")
	}
}

func TestFormatKuwoLrc(t *testing.T) {
	items := []KuwoLrcItem{
		{LineLyric: "七里香 - 周杰伦", Time: "0.0"},
		{LineLyric: "窗外的麻雀 在电线杆上多嘴", Time: "18.5"},
	}

	synced, plain := FormatKuwoLrc(items)
	if !strings.Contains(synced, "[00:18.50] 窗外的麻雀 在电线杆上多嘴") {
		t.Errorf("Unexpected synced output: %s", synced)
	}
	if !strings.Contains(plain, "窗外的麻雀 在电线杆上多嘴") {
		t.Errorf("Unexpected plain output: %s", plain)
	}
}
