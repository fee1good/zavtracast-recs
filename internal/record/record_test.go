package record

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSlug(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"The Legend of Zelda: Ocarina of Time", "the-legend-of-zelda-ocarina-of-time"},
		{"Пикник на обочине", "пикник-на-обочине"},
		{"guest:Иван", "guest-иван"},
		{"  --Half-Life 2!!  ", "half-life-2"},
		{"", ""},
		{"!!!", ""},
	}
	for _, tt := range tests {
		if got := Slug(tt.in); got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSlugLong(t *testing.T) {
	long := strings.Repeat("ab ", 100)
	got := Slug(long)
	if n := utf8.RuneCountInString(got); n > 80 {
		t.Errorf("len = %d runes, want <= 80", n)
	}
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Errorf("slug not trimmed: %q", got)
	}
	cyr := Slug(strings.Repeat("я", 200))
	if n := utf8.RuneCountInString(cyr); n != 80 {
		t.Errorf("cyrillic len = %d runes, want 80", n)
	}
}

func TestMakeID(t *testing.T) {
	got := MakeID(395, "The Legend of Zelda: Ocarina of Time", "maxim")
	if want := "395-the-legend-of-zelda-ocarina-of-time-maxim"; got != want {
		t.Errorf("id = %q, want %q", got, want)
	}
	if got := MakeID(1, "Пикник", "guest:Иван"); got != "1-пикник-guest-иван" {
		t.Errorf("guest id = %q", got)
	}
}

func TestValidHost(t *testing.T) {
	for _, h := range []string{"dima", "maxim", "timur", "unknown", "guest:Иван"} {
		if !ValidHost(h) {
			t.Errorf("ValidHost(%q) = false", h)
		}
	}
	for _, h := range []string{"", "team", "guest:", "guest:  ", "Dima", "max"} {
		if ValidHost(h) {
			t.Errorf("ValidHost(%q) = true", h)
		}
	}
}

func media() Record {
	return Record{
		Title:   "Ocarina of Time",
		Type:    "game",
		Host:    "maxim",
		Verdict: "positive",
		Summary: "понравилось",
		T:       100,
	}
}

func article() Record {
	return Record{
		Title:   "Статья",
		Type:    TypeArticle,
		Host:    HostTeam,
		Verdict: "positive",
		URL:     "https://example.com/a",
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		r    Record
		ok   bool
	}{
		{"media ok", media(), true},
		{"guest host", func() Record { r := media(); r.Host = "guest:Иван"; return r }(), true},
		{"article ok", article(), true},
		{"t zero", func() Record { r := media(); r.T = 0; return r }(), true},
		{"t equals duration", func() Record { r := media(); r.T = 1000; return r }(), true},
		{"bad type", func() Record { r := media(); r.Type = "podcast"; return r }(), false},
		{"empty type", func() Record { r := media(); r.Type = ""; return r }(), false},
		{"bad verdict", func() Record { r := media(); r.Verdict = "meh"; return r }(), false},
		{"bad host", func() Record { r := media(); r.Host = "vasya"; return r }(), false},
		{"team host on media", func() Record { r := media(); r.Host = HostTeam; return r }(), false},
		{"blank guest", func() Record { r := media(); r.Host = "guest: "; return r }(), false},
		{"blank summary", func() Record { r := media(); r.Summary = "  "; return r }(), false},
		{"url on media", func() Record { r := media(); r.URL = "https://x"; return r }(), false},
		{"blank title", func() Record { r := media(); r.Title = " \t"; return r }(), false},
		{"negative t", func() Record { r := media(); r.T = -1; return r }(), false},
		{"t over duration", func() Record { r := media(); r.T = 1001; return r }(), false},
		{"article host", func() Record { r := article(); r.Host = "dima"; return r }(), false},
		{"article no url", func() Record { r := article(); r.URL = ""; return r }(), false},
		{"article bad url", func() Record { r := article(); r.URL = "ftp://x"; return r }(), false},
		{"article bad verdict", func() Record { r := article(); r.Verdict = ""; return r }(), false},
		{"article blank title", func() Record { r := article(); r.Title = ""; return r }(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.r.Validate(1000)
			if tt.ok && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.ok && err == nil {
				t.Error("expected error")
			}
		})
	}
	for _, typ := range MediaTypes {
		r := media()
		r.Type = typ
		if err := r.Validate(1000); err != nil {
			t.Errorf("type %q: %v", typ, err)
		}
	}
}

func TestClean(t *testing.T) {
	dupe := media()
	dupe.Summary = "второй раз"
	bad := media()
	bad.Title = "Bad"
	bad.Verdict = "meh"
	other := media()
	other.Host = "dima"
	recs := []Record{media(), bad, dupe, other, article()}

	kept, dropped := Clean(395, 1000, recs)
	if len(kept) != 3 {
		t.Fatalf("kept = %d, want 3: %+v", len(kept), kept)
	}
	wantIDs := []string{
		"395-ocarina-of-time-maxim",
		"395-ocarina-of-time-dima",
		"395-статья-team",
	}
	for i, id := range wantIDs {
		if kept[i].ID != id {
			t.Errorf("kept[%d].ID = %q, want %q", i, kept[i].ID, id)
		}
	}
	if kept[0].Summary != "понравилось" {
		t.Errorf("first duplicate did not win: %q", kept[0].Summary)
	}
	if len(dropped) != 2 {
		t.Fatalf("dropped = %d, want 2: %v", len(dropped), dropped)
	}
	if msg := dropped[0].Error(); !strings.Contains(msg, "Bad") || !strings.Contains(msg, "verdict") {
		t.Errorf("dropped[0] = %q", msg)
	}
	if msg := dropped[1].Error(); !strings.Contains(msg, "Ocarina of Time") ||
		!strings.Contains(msg, "duplicate") {
		t.Errorf("dropped[1] = %q", msg)
	}
	if recs[0].ID != "" {
		t.Error("input slice mutated")
	}
}

func TestCleanEmpty(t *testing.T) {
	kept, dropped := Clean(1, 10, nil)
	if len(kept) != 0 || len(dropped) != 0 {
		t.Errorf("kept %v dropped %v", kept, dropped)
	}
}

func TestCheckDropped(t *testing.T) {
	tests := []struct {
		total, dropped int
		fail           bool
	}{
		{9, 3, false},
		{9, 4, true},
		{0, 0, false},
		{3, 1, false},
		{1, 1, true},
	}
	for _, tt := range tests {
		err := CheckDropped(tt.total, tt.dropped)
		if tt.fail != (err != nil) {
			t.Errorf("CheckDropped(%d, %d) = %v", tt.total, tt.dropped, err)
		}
		if tt.fail && !errors.Is(err, ErrTooManyInvalid) {
			t.Errorf("CheckDropped(%d, %d) not ErrTooManyInvalid: %v", tt.total, tt.dropped, err)
		}
	}
}
