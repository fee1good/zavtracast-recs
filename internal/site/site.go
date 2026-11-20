package site

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/fee1good/zavtracast-recs/internal/record"
)

type Row struct {
	record.Record
	Episode      int    `json:"episode"`
	EpisodeTitle string `json:"episode_title"`
	Date         string `json:"date"`
	PageURL      string `json:"page_url"`
	AudioURL     string `json:"audio_url"`
}

const marker = "<!--goatcounter-->"

var (
	staticFiles   = []string{"index.html", "app.js", "style.css"}
	goatcounterRe = regexp.MustCompile(`^[a-z0-9-]+$`)
)

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

func Build(siteDir, outDir string, eps []record.Episode, goatcounter string) error {
	snippet := ""
	if goatcounter != "" {
		if !goatcounterRe.MatchString(goatcounter) {
			return fmt.Errorf("invalid goatcounter code %q", goatcounter)
		}
		snippet = `<script data-goatcounter="https://` + goatcounter +
			`.goatcounter.com/count" async src="//gc.zgo.at/count.js"></script>`
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	for _, name := range staticFiles {
		b, err := os.ReadFile(filepath.Join(siteDir, name))
		if err != nil {
			return fmt.Errorf("read site file: %w", err)
		}
		if name == "index.html" {
			b = []byte(strings.ReplaceAll(string(b), marker, snippet))
		}
		if err := os.WriteFile(filepath.Join(outDir, name), b, 0o644); err != nil {
			return fmt.Errorf("write site file: %w", err)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(Rows(eps)); err != nil {
		return fmt.Errorf("encode data: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "data.json"), buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	return nil
}
