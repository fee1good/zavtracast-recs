package feed

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func parseFixture(t *testing.T) []Episode {
	t.Helper()
	f, err := os.Open("testdata/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	eps, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return eps
}

func TestParse(t *testing.T) {
	eps := parseFixture(t)
	nums := make([]int, 0, len(eps))
	for _, e := range eps {
		nums = append(nums, e.Number)
	}
	if !slices.Equal(nums, []int{394, 395}) {
		t.Fatalf("numbers = %v, want [394 395]", nums)
	}

	e := eps[1]
	if e.Duration != 8261 {
		t.Errorf("395 duration = %d, want 8261", e.Duration)
	}
	if e.PageURL != "https://zavtracast.ru/395.html" {
		t.Errorf("395 page url = %q", e.PageURL)
	}
	want := time.Date(2026, 11, 11, 17, 25, 38, 0, time.UTC)
	if !e.Date.Equal(want) {
		t.Errorf("395 date = %v, want %v", e.Date, want)
	}
	if !strings.HasPrefix(e.Title, "Завтракаст 395") {
		t.Errorf("395 title = %q", e.Title)
	}
	if len(e.Shownotes) != 8 {
		t.Errorf("395 shownotes = %d, want 8: %q", len(e.Shownotes), e.Shownotes)
	}
	if len(e.Impressions) != 3 {
		t.Fatalf("395 impressions = %d, want 3: %q", len(e.Impressions), e.Impressions)
	}
	last := e.Impressions[len(e.Impressions)-1]
	if !strings.HasPrefix(last, "Дима и Максим поиграли в The Legend of Zelda: Ocarina of Time") {
		t.Errorf("395 last impression = %q", last)
	}
	if len(e.Articles) != 0 {
		t.Errorf("395 articles = %v, want none", e.Articles)
	}
	if e.AudioURL == "" || !strings.Contains(e.AudioURL, ".mp3") {
		t.Errorf("395 audio url = %q", e.AudioURL)
	}

	e = eps[0]
	if e.Duration != 15074 {
		t.Errorf("394 duration = %d, want 15074", e.Duration)
	}
	if len(e.Shownotes) != 15 {
		t.Errorf("394 shownotes = %d, want 15: %q", len(e.Shownotes), e.Shownotes)
	}
	if len(e.Impressions) != 8 {
		t.Fatalf("394 impressions = %d, want 8: %q", len(e.Impressions), e.Impressions)
	}
	if e.Impressions[0] != "Дима про Steam Controller." {
		t.Errorf("394 first impression = %q", e.Impressions[0])
	}
	wantArt := []Article{{
		Title: "Как американцы нашли русский город на Аляске",
		URL:   "https://youtu.be/TMQ31DQjjWg?is=KMn2Gib7v4dC4wpH",
	}}
	if !slices.Equal(e.Articles, wantArt) {
		t.Errorf("394 articles = %q, want %q", e.Articles, wantArt)
	}
}

func TestParseHTML(t *testing.T) {
	content := `<p>intro</p>
<h2>Шоуноты</h2>
<ol><li><b>Тимур,</b> про &amp; книгу</li><li> </li><li>Максимум новостей</li></ol>
<h3>Лучшие материалы недели</h3>
<ol>
<li>Статья раз – <a href="https://a.example/x?a=1&amp;b=2">https://a.example/x</a></li>
<li>Без ссылки</li>
<li><a href="https://b.example/">https://b.example/</a></li>
</ol>`
	feed := `<?xml version="1.0"?>
<rss xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel>
<item><title>Завтракаст 7 – x</title><pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate>
<content:encoded><![CDATA[` + content + `]]></content:encoded></item>
<item><title>Завтракаст 70s</title><pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate></item>
</channel></rss>`
	eps, err := Parse(strings.NewReader(feed))
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("episodes = %d, want 1", len(eps))
	}
	e := eps[0]
	if !slices.Equal(e.Shownotes, []string{"Тимур, про & книгу", "Максимум новостей"}) {
		t.Errorf("shownotes = %q", e.Shownotes)
	}
	if !slices.Equal(e.Impressions, []string{"Тимур, про & книгу"}) {
		t.Errorf("impressions = %q", e.Impressions)
	}
	want := []Article{{Title: "Статья раз", URL: "https://a.example/x?a=1&b=2"}}
	if !slices.Equal(e.Articles, want) {
		t.Errorf("articles = %q, want %q", e.Articles, want)
	}
}

func TestParseListBoundedByHeading(t *testing.T) {
	content := `<h2>Шоуноты</h2>
<ul><li>Дима про список без нумерации</li></ul>
<h3>Самые интересные материалы недели</h3>
<p>пусто</p>
<h3>Прочее</h3>
<ol><li>Чужой – <a href="https://c.example/">https://c.example/</a></li></ol>`
	feed := `<rss xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel>
<item><title>Завтракаст 8</title><pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate>
<content:encoded><![CDATA[` + content + `]]></content:encoded></item>
</channel></rss>`
	eps, err := Parse(strings.NewReader(feed))
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("episodes = %d, want 1", len(eps))
	}
	if e := eps[0]; len(e.Shownotes) != 0 || len(e.Articles) != 0 {
		t.Errorf("shownotes = %q, articles = %q, want none", e.Shownotes, e.Articles)
	}
}

func TestParseKeepsOnlyHTTPURLs(t *testing.T) {
	item := func(n, link, audio string) string {
		return `<item><title>Завтракаст ` + n + `</title>` +
			`<pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate>` +
			`<link>` + link + `</link><enclosure url="` + audio + `"/></item>`
	}
	feed := `<rss><channel>` +
		item("1", " https://zavtracast.ru/1.html ", "http://media.example/1.mp3") +
		item("2", "javascript:alert(1)", "data:audio/mp3;base64,AA") +
		`</channel></rss>`
	eps, err := Parse(strings.NewReader(feed))
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("episodes = %d, want 2", len(eps))
	}
	if eps[0].PageURL != "https://zavtracast.ru/1.html" ||
		eps[0].AudioURL != "http://media.example/1.mp3" {
		t.Errorf("1 urls = %q %q", eps[0].PageURL, eps[0].AudioURL)
	}
	if eps[1].PageURL != "" || eps[1].AudioURL != "" {
		t.Errorf("2 urls = %q %q, want empty", eps[1].PageURL, eps[1].AudioURL)
	}
}

func TestParseErrors(t *testing.T) {
	wrap := func(body string) string {
		return `<rss xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"><channel>` +
			`<item><title>Завтракаст 1</title>` + body + `</item></channel></rss>`
	}
	date := `<pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate>`
	tests := map[string]string{
		"bad xml":      `<rss><channel><item>`,
		"bad date":     wrap(`<pubDate>x</pubDate>`),
		"bad duration": wrap(date + `<itunes:duration>1:99:00</itunes:duration>`),
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(src)); err == nil {
				t.Error("want error")
			}
		})
	}
}

func TestFetch(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "testdata/feed.xml")
	}))
	defer ok.Close()
	eps, err := Fetch(t.Context(), ok.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(eps) != 2 {
		t.Errorf("episodes = %d, want 2", len(eps))
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()
	if _, err := Fetch(t.Context(), bad.URL); err == nil {
		t.Error("want error on 500")
	}
}
