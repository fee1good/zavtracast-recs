package eval

import (
	"maps"
	"slices"

	"github.com/fee1good/zavtracast-recs/internal/record"
)

type Result struct {
	Found           []string
	Missed          []string
	Extra           []string
	VerdictMismatch []string
}

func key(r record.Record) string {
	return record.Slug(r.Title) + "|" + r.Host
}

func index(recs []record.Record) map[string]string {
	m := make(map[string]string)
	for _, r := range recs {
		if r.Type == record.TypeArticle {
			continue
		}
		k := key(r)
		if _, ok := m[k]; !ok {
			m[k] = r.Verdict
		}
	}
	return m
}

func Compare(golden, got []record.Record) Result {
	g, h := index(golden), index(got)
	var res Result
	for _, k := range slices.Sorted(maps.Keys(g)) {
		v, ok := h[k]
		if !ok {
			res.Missed = append(res.Missed, k)
			continue
		}
		res.Found = append(res.Found, k)
		if v != g[k] {
			res.VerdictMismatch = append(res.VerdictMismatch, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(h)) {
		if _, ok := g[k]; !ok {
			res.Extra = append(res.Extra, k)
		}
	}
	return res
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func (r Result) Precision() float64 {
	return ratio(len(r.Found), len(r.Found)+len(r.Extra))
}

func (r Result) Recall() float64 {
	return ratio(len(r.Found), len(r.Found)+len(r.Missed))
}
