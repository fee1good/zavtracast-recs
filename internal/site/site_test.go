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

const realSite = "../../site"

func episodes() []record.Episode {
	return []record.Episode{
		{
			Episode:  394,
			Title:    "Завтракаст 394",
			Date:     "2026-11-04",
			PageURL:  "https://zavtracast.ru/394.html",
			AudioURL: "https://media.example.com/394.mp3",
			Records: []record.Record{
				{ID: "394-b-dima", Title: "Bravo Game", Type: "game", Host: "dima",
					Verdict: "positive", Summary: "Понравилось", T: 300},
				{ID: "394-a-maxim", Title: "Alpha Movie", TitleRU: "Альфа", Type: "movie",
					Host: "maxim", Verdict: "mixed", Summary: "Так себе", T: 100},
			},
		},
		{
			Episode:  395,
			Title:    "Завтракаст 395 & <co>",
			Date:     "2026-11-11",
			PageURL:  "https://zavtracast.ru/395.html",
			AudioURL: "https://media.example.com/395.mp3?a=1&b=2",
			Records: []record.Record{
				{ID: "395-z-dima", Title: "Zulu Book", Type: "book", Host: "guest:Иван",
					Verdict: "negative", Summary: "Скучно", T: 50},
				{ID: "395-y-team", Title: "Yankee Article", Type: record.TypeArticle,
					Host: "team", Verdict: "positive", URL: "https://example.com/y?q=1&r=2"},
				{ID: "395-x-team", Title: "Xray Article", Type: record.TypeArticle,
					Host: "team", Verdict: "positive", URL: "https://example.com/x"},
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustContain(t *testing.T, name, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			t.Errorf("%s: missing %q", name, sub)
		}
	}
}

func mustNotContain(t *testing.T, name, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			t.Errorf("%s: unexpected %q", name, sub)
		}
	}
}

func build(t *testing.T, eps []record.Episode, o Options) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "_site")
	if err := Build(realSite, out, eps, o); err != nil {
		t.Fatalf("build: %v", err)
	}
	return out
}

func TestBuildHome(t *testing.T) {
	out := build(t, episodes(), Options{
		Goatcounter:        "zc-recs1",
		GoogleVerification: "g-Tok_1",
		YandexVerification: "y0tok",
	})
	for _, name := range []string{"app.js", "style.css"} {
		if got, want := readFile(t, filepath.Join(out, name)),
			readFile(t, filepath.Join(realSite, name)); got != want {
			t.Errorf("%s not copied verbatim", name)
		}
	}
	home := readFile(t, filepath.Join(out, "index.html"))
	mustContain(t, "index.html", home,
		`<link rel="canonical" href="https://fee1good.github.io/zavtracast-recs/">`,
		`<meta property="og:url" content="https://fee1good.github.io/zavtracast-recs/">`,
		`<meta name="description" content="Во что ведущие подкаста «Завтракаст» поиграли,`,
		`5 рекомендаций из 2 выпусков`,
		`<meta name="google-site-verification" content="g-Tok_1">`,
		`<meta name="yandex-verification" content="y0tok">`,
		`<script data-goatcounter="https://zc-recs1.goatcounter.com/count" async`,
		`<script src="app.js" defer></script>`,
		`<link rel="stylesheet" href="style.css">`,
		`"@type":"WebSite"`,
		`Записей: 5`,
		`<h2 class="title">Bravo Game</h2>`,
		`<h2 class="title">Alpha Movie</h2><span class="title-ru">Альфа</span>`,
		`<span class="tag">Иван (гость)</span>`,
		`<span class="tag verdict-negative">Не понравилось</span>`,
		`<a href="https://media.example.com/395.mp3?a=1&amp;b=2#t=50" target="_blank" `+
			`rel="noopener">▶ 00:00:50</a>`,
		`<a href="https://example.com/y?q=1&amp;r=2" target="_blank" rel="noopener">Открыть</a>`,
		`<a href="episodes/395/">Завтракаст 395 &amp; &lt;co&gt;</a>`,
		`<a href="episodes/394/">Завтракаст 394</a>`,
		`id="controls"`,
	)
	mustNotContain(t, "index.html", home, "& <co>", `class="crumbs"`)
	if strings.Index(home, "episodes/395/") > strings.Index(home, "episodes/394/") {
		t.Error("episode links not sorted newest first")
	}
}

func TestBuildEpisodePage(t *testing.T) {
	out := build(t, episodes(), Options{})
	p := readFile(t, filepath.Join(out, "episodes", "395", "index.html"))
	mustContain(t, "episode page", p,
		`<link rel="canonical" href="https://fee1good.github.io/zavtracast-recs/episodes/395/">`,
		`<title>Завтракаст 395 &amp; &lt;co&gt;: рекомендации ведущих | Рекомендации Завтракаста</title>`,
		`<link rel="stylesheet" href="../../style.css">`,
		`<a href="../../">Все рекомендации</a>`,
		`<a href="../../episodes/394/">`,
		`Zulu Book.`,
		`"@type":"ItemList"`,
		`"@type":"PodcastEpisode"`,
		`"episodeNumber":395`,
		`"@type":"Book"`,
		`"@type":"Article"`,
		`Записей: 3`,
		`(2026-11-11): 3 записи.`,
	)
	mustNotContain(t, "episode page", p, "app.js", `id="controls"`, "goatcounter",
		"verification", "Bravo Game")
	if strings.Index(p, "Zulu Book</h2>") > strings.Index(p, "Xray Article</h2>") {
		t.Error("articles must come after media records")
	}
	other := readFile(t, filepath.Join(out, "episodes", "394", "index.html"))
	mustContain(t, "episode 394", other, `"alternateName":"Альфа"`, `"@type":"Movie"`,
		"Alpha Movie, Bravo Game.")
}

func TestBuildData(t *testing.T) {
	out := build(t, episodes(), Options{})
	data := readFile(t, filepath.Join(out, "data.json"))
	if strings.Contains(data, "\\u00") || !strings.Contains(data, "a=1&b=2") ||
		!strings.Contains(data, "& <co>") {
		t.Errorf("data.json is html-escaped: %s", data)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(data), &rows); err != nil {
		t.Fatalf("decode data.json: %v", err)
	}
	if len(rows) != 5 || rows[0]["id"] != "395-x-team" || rows[0]["episode"] != float64(395) {
		t.Errorf("rows = %v", rows)
	}
	llms := readFile(t, filepath.Join(out, "llms.txt"))
	mustContain(t, "llms.txt", llms,
		"# Рекомендации Завтракаста\n\n> ",
		"Записей: 5, выпусков: 2.",
		"- [data.json](https://fee1good.github.io/zavtracast-recs/data.json)",
		"- [Завтракаст 395 & <co>](https://fee1good.github.io/zavtracast-recs/episodes/395/): "+
			"2026-11-11, записей: 3\n",
	)
	if strings.Index(llms, "episodes/395/") > strings.Index(llms, "episodes/394/") {
		t.Error("llms.txt episodes not sorted newest first")
	}
}

func TestBuildEmpty(t *testing.T) {
	out := build(t, nil, Options{})
	if got := readFile(t, filepath.Join(out, "data.json")); got != "[]\n" {
		t.Errorf("data.json = %q, want []", got)
	}
	home := readFile(t, filepath.Join(out, "index.html"))
	mustContain(t, "index.html", home, "Записей: 0", "0 рекомендаций из 0 выпусков")
	mustNotContain(t, "index.html", home, "goatcounter", "verification", `class="card"`)
	sm := readFile(t, filepath.Join(out, "sitemap.xml"))
	if strings.Count(sm, "<url>") != 1 {
		t.Errorf("sitemap = %s", sm)
	}
}

func TestJSONLDCannotCloseScript(t *testing.T) {
	eps := []record.Episode{{
		Episode: 1, Title: "T", Date: "2026-01-01",
		Records: []record.Record{{ID: "1-x-dima", Title: "</script><script>alert(1)</script>",
			Type: "game", Host: "dima", Verdict: "positive", Summary: "s", T: 1}},
	}}
	out := build(t, eps, Options{})
	for _, name := range []string{"index.html", "episodes/1/index.html"} {
		p := readFile(t, filepath.Join(out, name))
		mustNotContain(t, name, p, "<script>alert(1)")
		if strings.Count(p, "</script>") != strings.Count(p, "<script") {
			t.Errorf("%s: unbalanced script tags", name)
		}
	}
}

func TestSitemap(t *testing.T) {
	b, err := Sitemap("https://x.example/s/", episodes())
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://x.example/s/</loc>
    <lastmod>2026-11-11</lastmod>
  </url>
  <url>
    <loc>https://x.example/s/episodes/394/</loc>
    <lastmod>2026-11-04</lastmod>
  </url>
  <url>
    <loc>https://x.example/s/episodes/395/</loc>
    <lastmod>2026-11-11</lastmod>
  </url>
</urlset>
`
	if string(b) != want {
		t.Errorf("sitemap =\n%s\nwant\n%s", b, want)
	}
}

func TestBuildCustomBase(t *testing.T) {
	out := build(t, episodes(), Options{BaseURL: "https://recs.example/"})
	mustContain(t, "sitemap", readFile(t, filepath.Join(out, "sitemap.xml")),
		"<loc>https://recs.example/episodes/395/</loc>")
	mustContain(t, "index", readFile(t, filepath.Join(out, "index.html")),
		`<link rel="canonical" href="https://recs.example/">`)
}

func TestBuildRejectsBadOptions(t *testing.T) {
	bad := []Options{
		{Goatcounter: "Bad"},
		{Goatcounter: `x"><script>`},
		{Goatcounter: "a.b"},
		{GoogleVerification: `x"><script>`},
		{YandexVerification: "a b"},
		{BaseURL: "http://insecure.example/"},
		{BaseURL: "https://no-slash.example"},
	}
	for _, o := range bad {
		out := filepath.Join(t.TempDir(), "out")
		if err := Build(realSite, out, nil, o); err == nil {
			t.Errorf("%+v: no error", o)
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("%+v: out dir created", o)
		}
	}
}

func TestBuildMissingTemplate(t *testing.T) {
	if err := Build(t.TempDir(), t.TempDir(), nil, Options{}); err == nil {
		t.Error("no error for missing site files")
	}
}

func TestPlural(t *testing.T) {
	cases := map[int]string{
		0: "записей", 1: "запись", 2: "записи", 4: "записи", 5: "записей", 11: "записей",
		12: "записей", 14: "записей", 21: "запись", 22: "записи", 111: "записей",
		112: "записей", 123: "записи", 1001: "запись",
	}
	for n, want := range cases {
		if got := Plural(n, "запись", "записи", "записей"); got != want {
			t.Errorf("Plural(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestLabelsCoverAllValues(t *testing.T) {
	for _, k := range append(slices.Clone(record.MediaTypes), record.TypeArticle) {
		if TypeLabels[k] == "" {
			t.Errorf("no type label for %q", k)
		}
	}
	for _, k := range record.Verdicts {
		if VerdictLabels[k] == "" {
			t.Errorf("no verdict label for %q", k)
		}
	}
	for _, k := range append(slices.Clone(record.Hosts), record.HostTeam) {
		if HostLabels[k] == "" {
			t.Errorf("no host label for %q", k)
		}
	}
}

func TestAppLabelsMatchGo(t *testing.T) {
	src := readFile(t, filepath.Join(realSite, "app.js"))
	for _, labels := range []map[string]string{TypeLabels, HostLabels, VerdictLabels} {
		for k, v := range labels {
			if want := "\n  " + k + `: "` + v + `",`; !strings.Contains(src, want) {
				t.Errorf("site/app.js: no %q", strings.TrimSpace(want))
			}
		}
	}
}
