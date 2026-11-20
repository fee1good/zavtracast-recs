package record

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type Episode struct {
	Episode  int      `json:"episode"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	PageURL  string   `json:"page_url"`
	AudioURL string   `json:"audio_url"`
	Duration int      `json:"duration"`
	Records  []Record `json:"records"`
}

const DateLayout = "2006-01-02"

func Path(dir string, n int) string {
	return filepath.Join(dir, strconv.Itoa(n)+".json")
}

func Exists(dir string, n int) bool {
	st, err := os.Stat(Path(dir, n))
	return err == nil && !st.IsDir()
}

func Write(dir string, e Episode) error {
	if e.Records == nil {
		e.Records = []Record{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(e); err != nil {
		return fmt.Errorf("encode episode %d: %w", e.Episode, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.WriteFile(Path(dir, e.Episode), buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write episode %d: %w", e.Episode, err)
	}
	return nil
}

func Read(dir string, n int) (Episode, error) {
	b, err := os.ReadFile(Path(dir, n))
	if err != nil {
		return Episode{}, fmt.Errorf("read episode %d: %w", n, err)
	}
	var e Episode
	if err := json.Unmarshal(b, &e); err != nil {
		return Episode{}, fmt.Errorf("decode episode %d: %w", n, err)
	}
	return e, nil
}

func ReadAll(dir string) ([]Episode, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read data dir: %w", err)
	}
	var eps []Episode
	for _, ent := range entries {
		name, ok := strings.CutSuffix(ent.Name(), ".json")
		if !ok || ent.IsDir() {
			continue
		}
		n, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		e, err := Read(dir, n)
		if err != nil {
			return nil, err
		}
		eps = append(eps, e)
	}
	slices.SortFunc(eps, func(a, b Episode) int { return cmp.Compare(a.Episode, b.Episode) })
	return eps, nil
}
