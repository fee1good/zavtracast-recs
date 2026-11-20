package site

import (
	"bytes"
	"cmp"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/fee1good/zavtracast-recs/internal/clock"
	"github.com/fee1good/zavtracast-recs/internal/record"
)

const (
	DefaultBaseURL = "https://fee1good.github.io/zavtracast-recs/"
	siteName       = "Рекомендации Завтракаста"
	pageTemplate   = "page.html"
)

type Row struct {
	record.Record
	Episode      int    `json:"episode"`
	EpisodeTitle string `json:"episode_title"`
	Date         string `json:"date"`
	PageURL      string `json:"page_url"`
	AudioURL     string `json:"audio_url"`
}

type Options struct {
	BaseURL            string
	Goatcounter        string
	GoogleVerification string
	YandexVerification string
}

var (
	staticFiles = []string{"app.js", "style.css"}
	codeRe      = regexp.MustCompile(`^[a-z0-9-]+$`)
	tokenRe     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

var TypeLabels = map[string]string{
	"game":             "Игра",
	"movie":            "Фильм",
	"series":           "Сериал",
	"anime":            "Аниме",
	"cartoon":          "Мультфильм",
	"book":             "Книга",
	"comic":            "Комикс",
	"other":            "Другое",
	record.TypeArticle: "Статья",
}

var HostLabels = map[string]string{
	"dima":          "Дима",
	"maxim":         "Максим",
	"timur":         "Тимур",
	record.HostTeam: "Редакция",
	"unknown":       "Неизвестно",
}

var VerdictLabels = map[string]string{
	"positive": "Нравится",
	"mixed":    "Смешанно",
	"negative": "Не понравилось",
}

var schemaTypes = map[string]string{
	"game":             "VideoGame",
	"movie":            "Movie",
	"series":           "TVSeries",
	"book":             "Book",
	"comic":            "ComicStory",
	"other":            "Thing",
	record.TypeArticle: "Article",
}

func Rows(eps []record.Episode) []Row {
	rows := []Row{}
	for _, e := range eps {
		for _, r := range e.Records {
			rows = append(rows, Row{
				Record:       r,
				Episode:      e.Episode,
				EpisodeTitle: e.Title,
				Date:         e.Date,
				PageURL:      e.PageURL,
				AudioURL:     e.AudioURL,
			})
		}
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		return cmp.Or(
			cmp.Compare(b.Episode, a.Episode),
			cmp.Compare(a.T, b.T),
			strings.Compare(a.ID, b.ID),
		)
	})
	return rows
}

func HostLabel(host string) string {
	if name, ok := strings.CutPrefix(host, "guest:"); ok {
		return name + " (гость)"
	}
	return cmp.Or(HostLabels[host], host)
}

type link struct {
	Href string
	Text string
}

type card struct {
	Title        string
	TitleRU      string
	TypeLabel    string
	HostLabel    string
	Verdict      string
	VerdictLabel string
	Summary      string
	Links        []link
}

type episodeLink struct {
	Number int
	Title  string
	Date   string
	Count  int
}

type page struct {
	Root               string
	URL                string
	SiteName           string
	Title              string
	Description        string
	Heading            string
	Lead               string
	Home               bool
	Cards              []card
	Episodes           []episodeLink
	JSONLD             template.JS
	Goatcounter        string
	GoogleVerification string
	YandexVerification string
}

func cardOf(r Row) card {
	c := card{
		Title:        r.Title,
		TitleRU:      r.TitleRU,
		TypeLabel:    cmp.Or(TypeLabels[r.Type], r.Type),
		HostLabel:    HostLabel(r.Host),
		Verdict:      r.Verdict,
		VerdictLabel: cmp.Or(VerdictLabels[r.Verdict], r.Verdict),
		Summary:      r.Summary,
	}
	if r.PageURL != "" {
		c.Links = append(c.Links, link{r.PageURL, "Выпуск " + strconv.Itoa(r.Episode)})
	}
	switch {
	case r.Type == record.TypeArticle:
		if r.URL != "" {
			c.Links = append(c.Links, link{r.URL, "Открыть"})
		}
	case r.AudioURL != "":
		c.Links = append(c.Links, link{
			r.AudioURL + "#t=" + strconv.Itoa(r.T), "▶ " + clock.Format(r.T),
		})
	}
	return c
}

func (o Options) validate() error {
	if o.Goatcounter != "" && !codeRe.MatchString(o.Goatcounter) {
		return fmt.Errorf("invalid goatcounter code %q", o.Goatcounter)
	}
	for _, v := range []string{o.GoogleVerification, o.YandexVerification} {
		if v != "" && !tokenRe.MatchString(v) {
			return fmt.Errorf("invalid verification token %q", v)
		}
	}
	if !strings.HasPrefix(o.BaseURL, "https://") || !strings.HasSuffix(o.BaseURL, "/") {
		return fmt.Errorf("base url must be https and end with /: %q", o.BaseURL)
	}
	return nil
}

func Build(siteDir, outDir string, eps []record.Episode, o Options) error {
	o.BaseURL = cmp.Or(o.BaseURL, DefaultBaseURL)
	if err := o.validate(); err != nil {
		return err
	}
	tmpl, err := template.ParseFiles(filepath.Join(siteDir, pageTemplate))
	if err != nil {
		return fmt.Errorf("parse page template: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	for _, name := range staticFiles {
		b, err := os.ReadFile(filepath.Join(siteDir, name))
		if err != nil {
			return fmt.Errorf("read site file: %w", err)
		}
		if err := writeFile(outDir, name, b); err != nil {
			return err
		}
	}

	eps = slices.SortedFunc(slices.Values(eps), func(a, b record.Episode) int {
		return cmp.Compare(b.Episode, a.Episode)
	})
	rows := Rows(eps)
	links := episodeLinks(eps)

	home, err := homePage(o, rows, links, len(eps))
	if err != nil {
		return err
	}
	if err := render(tmpl, outDir, "index.html", home); err != nil {
		return err
	}
	for _, e := range eps {
		p, err := episodePage(o, e, links)
		if err != nil {
			return err
		}
		name := filepath.Join("episodes", strconv.Itoa(e.Episode), "index.html")
		if err := render(tmpl, outDir, name, p); err != nil {
			return err
		}
	}

	data, err := encodeJSON(rows, false)
	if err != nil {
		return fmt.Errorf("encode data: %w", err)
	}
	if err := writeFile(outDir, "data.json", data); err != nil {
		return err
	}
	sitemap, err := Sitemap(o.BaseURL, eps)
	if err != nil {
		return err
	}
	if err := writeFile(outDir, "sitemap.xml", sitemap); err != nil {
		return err
	}
	return writeFile(outDir, "llms.txt", []byte(LLMs(o.BaseURL, eps, len(rows))))
}

func episodeLinks(eps []record.Episode) []episodeLink {
	links := make([]episodeLink, 0, len(eps))
	for _, e := range eps {
		links = append(links, episodeLink{e.Episode, e.Title, e.Date, len(e.Records)})
	}
	return links
}

func homePage(o Options, rows []Row, links []episodeLink, episodes int) (page, error) {
	desc := fmt.Sprintf("Во что ведущие подкаста «Завтракаст» поиграли, что посмотрели и "+
		"прочитали: %d %s из %d %s с оценками и таймкодами.",
		len(rows), Plural(len(rows), "рекомендация", "рекомендации", "рекомендаций"),
		episodes, Plural(episodes, "выпуска", "выпусков", "выпусков"))
	ld, err := encodeJSON(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebSite",
		"name":        siteName,
		"url":         o.BaseURL,
		"description": desc,
		"inLanguage":  "ru",
	}, true)
	if err != nil {
		return page{}, fmt.Errorf("encode json-ld: %w", err)
	}
	p := basePage(o, "")
	p.URL = o.BaseURL
	p.Title = siteName
	p.Description = desc
	p.Heading = siteName
	p.Lead = "Во что ведущие поиграли, что посмотрели и прочитали и что им понравилось."
	p.Home = true
	p.Episodes = links
	p.JSONLD = template.JS(bytes.TrimSpace(ld))
	for _, r := range rows {
		p.Cards = append(p.Cards, cardOf(r))
	}
	return p, nil
}

func episodePage(o Options, e record.Episode, links []episodeLink) (page, error) {
	rows := Rows([]record.Episode{e})
	slices.SortStableFunc(rows, func(a, b Row) int {
		return cmp.Compare(articleRank(a), articleRank(b))
	})
	url := o.BaseURL + "episodes/" + strconv.Itoa(e.Episode) + "/"
	heading := e.Title + ": рекомендации ведущих"

	var names []string
	for _, r := range rows {
		if articleRank(r) == 0 && len(names) < 5 {
			names = append(names, r.Title)
		}
	}
	desc := fmt.Sprintf("Что ведущие посоветовали в выпуске «%s» (%s): %d %s.",
		e.Title, e.Date, len(rows), Plural(len(rows), "запись", "записи", "записей"))
	if len(names) > 0 {
		desc += " " + strings.Join(names, ", ") + "."
	}

	ld, err := encodeJSON(episodeLD(e, rows, url, heading), true)
	if err != nil {
		return page{}, fmt.Errorf("encode json-ld: %w", err)
	}
	p := basePage(o, "../../")
	p.URL = url
	p.Title = heading + " | " + siteName
	p.Description = desc
	p.Heading = heading
	p.Lead = fmt.Sprintf("Выпуск от %s. Игры, фильмы, сериалы и статьи, о которых "+
		"рассказали в выпуске, с оценками и таймкодами.", e.Date)
	p.Episodes = links
	p.JSONLD = template.JS(bytes.TrimSpace(ld))
	for _, r := range rows {
		p.Cards = append(p.Cards, cardOf(r))
	}
	return p, nil
}

func episodeLD(e record.Episode, rows []Row, url, name string) map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for i, r := range rows {
		item := map[string]any{
			"@type": cmp.Or(schemaTypes[r.Type], "CreativeWork"),
			"name":  r.Title,
		}
		if r.TitleRU != "" {
			item["alternateName"] = r.TitleRU
		}
		if r.URL != "" {
			item["url"] = r.URL
		}
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"item":     item,
		})
	}
	episode := map[string]any{
		"@type":         "PodcastEpisode",
		"name":          e.Title,
		"episodeNumber": e.Episode,
		"datePublished": e.Date,
		"url":           e.PageURL,
		"partOfSeries": map[string]any{
			"@type": "PodcastSeries",
			"name":  "Завтракаст",
			"url":   "https://zavtracast.ru",
		},
	}
	if e.AudioURL != "" {
		episode["associatedMedia"] = map[string]any{
			"@type":      "AudioObject",
			"contentUrl": e.AudioURL,
		}
	}
	return map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type":           "ItemList",
				"name":            name,
				"url":             url,
				"numberOfItems":   len(items),
				"itemListElement": items,
			},
			episode,
		},
	}
}

func Plural(n int, one, few, many string) string {
	switch n10, n100 := n%10, n%100; {
	case n10 == 1 && n100 != 11:
		return one
	case n10 >= 2 && n10 <= 4 && (n100 < 12 || n100 > 14):
		return few
	default:
		return many
	}
}

func articleRank(r Row) int {
	if r.Type == record.TypeArticle {
		return 1
	}
	return 0
}

func basePage(o Options, root string) page {
	return page{
		Root:               root,
		SiteName:           siteName,
		Goatcounter:        o.Goatcounter,
		GoogleVerification: o.GoogleVerification,
		YandexVerification: o.YandexVerification,
	}
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func Sitemap(base string, eps []record.Episode) ([]byte, error) {
	set := urlset{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	latest := ""
	for _, e := range eps {
		latest = max(latest, e.Date)
	}
	set.URLs = append(set.URLs, sitemapURL{Loc: base, LastMod: latest})
	for _, e := range eps {
		set.URLs = append(set.URLs, sitemapURL{
			Loc:     base + "episodes/" + strconv.Itoa(e.Episode) + "/",
			LastMod: e.Date,
		})
	}
	b, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode sitemap: %w", err)
	}
	return append([]byte(xml.Header), append(b, '\n')...), nil
}

func LLMs(base string, eps []record.Episode, records int) string {
	var b strings.Builder
	b.WriteString("# " + siteName + "\n\n")
	b.WriteString("> Неофициальная база рекомендаций подкаста «Завтракаст»: игры, фильмы, " +
		"сериалы, аниме, книги и статьи, о которых ведущие рассказали в выпусках, " +
		"с оценкой и таймкодом. Записи извлечены автоматически из транскриптов " +
		"выпусков и могут содержать ошибки.\n\n")
	fmt.Fprintf(&b, "Записей: %d, выпусков: %d. Сайт обновляется после выхода "+
		"каждого нового выпуска.\n\n", records, len(eps))
	b.WriteString("## Данные\n\n")
	fmt.Fprintf(&b, "- [data.json](%sdata.json): все записи одним JSON-массивом. "+
		"Поля: `title`, `title_ru`, `type` (game, movie, series, anime, cartoon, book, "+
		"comic, other, article), `host` (dima, maxim, timur, team, unknown, guest:<имя>), "+
		"`verdict` (positive, mixed, negative), `summary`, `url` (у статей), "+
		"`t` (секунда начала обсуждения в mp3), `episode`, `episode_title`, `date`, "+
		"`page_url`, `audio_url`.\n", base)
	fmt.Fprintf(&b, "- [sitemap.xml](%ssitemap.xml)\n\n", base)
	b.WriteString("## Выпуски\n\n")
	for _, e := range eps {
		fmt.Fprintf(&b, "- [%s](%sepisodes/%d/): %s, записей: %d\n",
			e.Title, base, e.Episode, e.Date, len(e.Records))
	}
	return b.String()
}

func render(tmpl *template.Template, outDir, name string, p page) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return writeFile(outDir, name, buf.Bytes())
}

func encodeJSON(v any, escapeHTML bool) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(escapeHTML)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeFile(outDir, name string, b []byte) error {
	path := filepath.Join(outDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", name, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
