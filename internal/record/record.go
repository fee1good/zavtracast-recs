package record

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

type Record struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	TitleRU string `json:"title_ru,omitzero"`
	Type    string `json:"type"`
	Host    string `json:"host"`
	Verdict string `json:"verdict"`
	Summary string `json:"summary,omitzero"`
	URL     string `json:"url,omitzero"`
	T       int    `json:"t"`
}

var MediaTypes = []string{"game", "movie", "series", "anime", "cartoon", "book", "comic", "other"}

const TypeArticle = "article"

var Verdicts = []string{"positive", "mixed", "negative"}

var Hosts = []string{"dima", "maxim", "timur", "unknown"}

const HostTeam = "team"

var ErrTooManyInvalid = errors.New("too many invalid records")

const maxSlugRunes = 80

func ValidHost(h string) bool {
	if slices.Contains(Hosts, h) {
		return true
	}
	name, ok := strings.CutPrefix(h, "guest:")
	return ok && strings.TrimSpace(name) != ""
}

func (r Record) Validate(duration int) error {
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("empty title")
	}
	if !slices.Contains(Verdicts, r.Verdict) {
		return fmt.Errorf("bad verdict %q", r.Verdict)
	}
	switch {
	case r.Type == TypeArticle:
		if r.Host != HostTeam {
			return fmt.Errorf("bad article host %q", r.Host)
		}
		if !strings.HasPrefix(r.URL, "http") {
			return fmt.Errorf("bad article url %q", r.URL)
		}
	case slices.Contains(MediaTypes, r.Type):
		if !ValidHost(r.Host) {
			return fmt.Errorf("bad host %q", r.Host)
		}
		if strings.TrimSpace(r.Summary) == "" {
			return errors.New("empty summary")
		}
		if r.URL != "" {
			return errors.New("url on non-article")
		}
	default:
		return fmt.Errorf("bad type %q", r.Type)
	}
	if r.T < 0 || r.T > duration {
		return fmt.Errorf("time %d out of range 0..%d", r.T, duration)
	}
	return nil
}

func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if runes := []rune(out); len(runes) > maxSlugRunes {
		out = strings.Trim(string(runes[:maxSlugRunes]), "-")
	}
	return out
}

func MakeID(ep int, title, host string) string {
	return fmt.Sprintf("%d-%s-%s", ep, Slug(title), Slug(host))
}

func Clean(ep, duration int, recs []Record) (kept []Record, dropped []error) {
	seen := make(map[string]bool, len(recs))
	for _, r := range recs {
		r.ID = MakeID(ep, r.Title, r.Host)
		if err := r.Validate(duration); err != nil {
			dropped = append(dropped, fmt.Errorf("drop %q: %w", r.Title, err))
			continue
		}
		if seen[r.ID] {
			dropped = append(dropped, fmt.Errorf("drop %q: duplicate id %s", r.Title, r.ID))
			continue
		}
		seen[r.ID] = true
		kept = append(kept, r)
	}
	return kept, dropped
}

func CheckDropped(total, dropped int) error {
	if dropped*3 > total {
		return fmt.Errorf("%w: %d of %d", ErrTooManyInvalid, dropped, total)
	}
	return nil
}
