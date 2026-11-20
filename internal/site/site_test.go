package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fee1good/zavtracast-recs/internal/record"
)

func episodes() []record.Episode {
	return []record.Episode{
		{
			Episode:  394,
			Title:    "Завтракаст 394",
			Date:     "2026-11-04",
			PageURL:  "https://zavtracast.ru/394.html",
			AudioURL: "https://media.example.com/394.mp3",
			Records: []record.Record{
				{ID: "394-b-dima", Title: "B", Type: "game", Host: "dima", T: 300},
				{ID: "394-a-maxim", Title: "A", Type: "game", Host: "maxim", T: 100},
			},
		},
		{
			Episode:  395,
			Title:    "Завтракаст 395 & <co>",
			Date:     "2026-11-11",
			PageURL:  "https://zavtracast.ru/395.html",
			AudioURL: "https://media.example.com/395.mp3?a=1&b=2",
			Records: []record.Record{
				{ID: "395-z-dima", Title: "Z", Type: "book", Host: "dima", T: 50},
				{ID: "395-y-team", Title: "Y", Type: record.TypeArticle, Host: "team"},
				{ID: "395-x-team", Title: "X", Type: record.TypeArticle, Host: "team"},
			},
		},
	}
}

func TestRowsOrder(t *testing.T) {
	rows := Rows(episodes())
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	want := []string{"395-x-team", "395-y-team", "395-z-dima", "394-a-maxim", "394-b-dima"}
	if !slices.Equal(ids, want) {
		t.Fatalf("order = %v, want %v", ids, want)
	}
	r := rows[2]
	if r.Episode != 395 || r.EpisodeTitle != "Завтракаст 395 & <co>" || r.Date != "2026-11-11" ||
		r.PageURL != "https://zavtracast.ru/395.html" ||
		r.AudioURL != "https://media.example.com/395.mp3?a=1&b=2" {
		t.Errorf("episode fields = %+v", r)
	}
}

func TestRowsEmpty(t *testing.T) {
	if rows := Rows(nil); rows == nil || len(rows) != 0 {
		t.Errorf("rows = %#v, want empty non-nil", rows)
	}
}

func writeSite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html": "<head>\n<!--goatcounter-->\n</head>\n",
		"app.js":     "console.log(1)\n",
		"style.css":  "body{}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuildWithGoatcounter(t *testing.T) {
	siteDir := writeSite(t)
	out := filepath.Join(t.TempDir(), "_site")
	if err := Build(siteDir, out, episodes(), "zc-recs1"); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := readFile(t, filepath.Join(out, "app.js")); got != "console.log(1)\n" {
		t.Errorf("app.js = %q", got)
	}
	if got := readFile(t, filepath.Join(out, "style.css")); got != "body{}\n" {
		t.Errorf("style.css = %q", got)
	}
	want := "<head>\n<script data-goatcounter=\"https://zc-recs1.goatcounter.com/count\" " +
		"async src=\"//gc.zgo.at/count.js\"></script>\n</head>\n"
	if got := readFile(t, filepath.Join(out, "index.html")); got != want {
		t.Errorf("index.html = %q, want %q", got, want)
	}

	data := readFile(t, filepath.Join(out, "data.json"))
	if strings.Contains(data, "\\u00") || !strings.Contains(data, "a=1&b=2") ||
		!strings.Contains(data, "& <co>") {
		t.Errorf("data.json is html-escaped: %s", data)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(data), &rows); err != nil {
		t.Fatalf("decode data.json: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(rows))
	}
	first := rows[0]
	if first["id"] != "395-x-team" || first["episode"] != float64(395) ||
		first["episode_title"] != "Завтракаст 395 & <co>" || first["type"] != "article" {
		t.Errorf("first row = %v", first)
	}
}

func TestBuildWithoutGoatcounter(t *testing.T) {
	siteDir := writeSite(t)
	out := t.TempDir()
	if err := Build(siteDir, out, nil, ""); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := readFile(t, filepath.Join(out, "index.html")); got != "<head>\n\n</head>\n" {
		t.Errorf("index.html = %q", got)
	}
	if got := readFile(t, filepath.Join(out, "data.json")); got != "[]\n" {
		t.Errorf("data.json = %q, want []", got)
	}
}

func TestBuildRejectsBadCode(t *testing.T) {
	siteDir := writeSite(t)
	for _, code := range []string{"Bad", "a b", `x"><script>`, "a.b"} {
		out := filepath.Join(t.TempDir(), "out")
		if err := Build(siteDir, out, nil, code); err == nil {
			t.Errorf("code %q: no error", code)
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("code %q: out dir created", code)
		}
	}
}

func TestBuildMissingSiteFile(t *testing.T) {
	if err := Build(t.TempDir(), t.TempDir(), nil, ""); err == nil {
		t.Error("no error for missing site files")
	}
}
