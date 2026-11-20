package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fee1good/zavtracast-recs/internal/clock"
)

type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type Transcript struct {
	Episode  int       `json:"episode"`
	Model    string    `json:"model"`
	Duration float64   `json:"duration"`
	Segments []Segment `json:"segments"`
}

func Path(dir string, n int) string {
	return filepath.Join(dir, strconv.Itoa(n)+".json")
}

func Exists(dir string, n int) bool {
	st, err := os.Stat(Path(dir, n))
	return err == nil && !st.IsDir()
}

func Load(dir string, n int) (Transcript, error) {
	b, err := os.ReadFile(Path(dir, n))
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript %d: %w", n, err)
	}
	var tr Transcript
	if err := json.Unmarshal(b, &tr); err != nil {
		return Transcript{}, fmt.Errorf("decode transcript %d: %w", n, err)
	}
	return tr, nil
}

func (t Transcript) Lines() string {
	var b strings.Builder
	for _, s := range t.Segments {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "[%s] %s\n", clock.Format(int(s.Start)), text)
	}
	return b.String()
}
