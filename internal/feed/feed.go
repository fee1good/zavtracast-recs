package feed

import (
	"cmp"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fee1good/zavtracast-recs/internal/clock"
)

const URL = "https://feeds.podcastmirror.com/zavtracast"

type Article struct {
	Title, URL string
}

type Episode struct {
	Number      int
	Title       string
	Date        time.Time
	PageURL     string
	AudioURL    string
	Duration    int
	Shownotes   []string
	Impressions []string
	Articles    []Article
}

type rss struct {
	Items []item `xml:"channel>item"`
}

type item struct {
	Title     string `xml:"title"`
	Link      string `xml:"link"`
	PubDate   string `xml:"pubDate"`
	Enclosure struct {
		URL string `xml:"url,attr"`
	} `xml:"enclosure"`
	Duration string `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration"`
	Content  string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

var (
	titleRe      = regexp.MustCompile(`^Завтракаст (\d+)\b`)
	headingRe    = regexp.MustCompile(`(?s)<h[1-6][^>]*>(.*?)</h[1-6]>`)
	liRe         = regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`)
	tagRe        = regexp.MustCompile(`(?s)<[^>]*>`)
	hrefRe       = regexp.MustCompile(`href\s*=\s*["']([^"']*)["']`)
	bareURLRe    = regexp.MustCompile(`https?://\S+`)
	impressionRe = regexp.MustCompile(`^(Дима|Максим|Макс|Тимур)[\s,]`)
)

func Parse(r io.Reader) ([]Episode, error) {
	var doc rss
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode feed: %w", err)
	}
	var eps []Episode
	for _, it := range doc.Items {
		title := strings.TrimSpace(it.Title)
		m := titleRe.FindStringSubmatch(title)
		if m == nil {
			continue
		}
		ep, err := toEpisode(it, title, m[1])
		if err != nil {
			return nil, err
		}
		eps = append(eps, ep)
	}
	slices.SortFunc(eps, func(a, b Episode) int { return cmp.Compare(a.Number, b.Number) })
	return eps, nil
}

func toEpisode(it item, title, num string) (Episode, error) {
	n, err := strconv.Atoi(num)
	if err != nil {
		return Episode{}, fmt.Errorf("episode number in %q: %w", title, err)
	}
	date, err := time.Parse(time.RFC1123Z, strings.TrimSpace(it.PubDate))
	if err != nil {
		return Episode{}, fmt.Errorf("pub date of %q: %w", title, err)
	}
	var dur int
	if d := strings.TrimSpace(it.Duration); d != "" {
		if dur, err = clock.Parse(d); err != nil {
			return Episode{}, fmt.Errorf("duration of %q: %w", title, err)
		}
	}
	notes := shownotes(it.Content)
	return Episode{
		Number:      n,
		Title:       title,
		Date:        date,
		PageURL:     httpURL(it.Link),
		AudioURL:    httpURL(it.Enclosure.URL),
		Duration:    dur,
		Shownotes:   notes,
		Impressions: impressions(notes),
		Articles:    articles(it.Content),
	}, nil
}

func httpURL(s string) string {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return s
}

func listAfter(src, keyword string) string {
	headings := headingRe.FindAllStringSubmatchIndex(src, -1)
	for i, loc := range headings {
		if !strings.Contains(text(src[loc[2]:loc[3]]), keyword) {
			continue
		}
		end := len(src)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		rest := src[loc[1]:end]
		start := strings.Index(rest, "<ol")
		if start < 0 {
			return ""
		}
		list, _, _ := strings.Cut(rest[start:], "</ol>")
		return list
	}
	return ""
}

func items(list string) []string {
	var out []string
	for _, m := range liRe.FindAllStringSubmatch(list, -1) {
		out = append(out, m[1])
	}
	return out
}

func text(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func shownotes(content string) []string {
	var out []string
	for _, li := range items(listAfter(content, "Шоуноты")) {
		if t := text(li); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func impressions(notes []string) []string {
	var out []string
	for _, n := range notes {
		if impressionRe.MatchString(n) {
			out = append(out, n)
		}
	}
	return out
}

func articles(content string) []Article {
	var out []Article
	for _, li := range items(listAfter(content, "материалы недели")) {
		m := hrefRe.FindStringSubmatch(li)
		if m == nil {
			continue
		}
		u := strings.TrimSpace(html.UnescapeString(m[1]))
		t := strings.Join(strings.Fields(bareURLRe.ReplaceAllString(text(li), " ")), " ")
		t = strings.TrimSpace(strings.TrimRight(t, ":-– "))
		if u == "" || t == "" {
			continue
		}
		out = append(out, Article{Title: t, URL: u})
	}
	return out
}

func Fetch(ctx context.Context, rawURL string) ([]Episode, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("fetch feed: status %d", resp.StatusCode)
	}
	return Parse(resp.Body)
}
