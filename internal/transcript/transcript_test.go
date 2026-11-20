package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `{
  "episode": 395,
  "model": "large-v3-turbo",
  "duration": 8261.5,
  "segments": [
    {"start": 0.0, "end": 4.2, "text": " привет это завтракаст "},
    {"start": 4.2, "end": 5.0, "text": "   "},
    {"start": 3725.87, "end": 3730.1, "text": "я прошёл ocarina of time"}
  ]
}
`

func TestPath(t *testing.T) {
	if got, want := Path("x", 7), filepath.Join("x", "7.json"); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestLoadAndExists(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir, 395) {
		t.Fatal("exists before write")
	}
	if err := os.WriteFile(Path(dir, 395), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Exists(dir, 395) {
		t.Fatal("not exists after write")
	}
	if Exists(dir, 394) {
		t.Error("394 exists")
	}

	tr, err := Load(dir, 395)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tr.Episode != 395 || tr.Model != "large-v3-turbo" || tr.Duration != 8261.5 {
		t.Errorf("header = %d %q %v", tr.Episode, tr.Model, tr.Duration)
	}
	if len(tr.Segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(tr.Segments))
	}
	if s := tr.Segments[2]; s.Start != 3725.87 || s.End != 3730.1 {
		t.Errorf("segment 2 = %+v", s)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, 1); err == nil {
		t.Error("missing file: no error")
	}
	if err := os.WriteFile(Path(dir, 2), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, 2); err == nil {
		t.Error("bad json: no error")
	}
}

func TestExistsDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(Path(dir, 5), 0o755); err != nil {
		t.Fatal(err)
	}
	if Exists(dir, 5) {
		t.Error("directory reported as transcript")
	}
}

func TestLines(t *testing.T) {
	tr := Transcript{Segments: []Segment{
		{Start: 0, End: 4.2, Text: " привет это завтракаст "},
		{Start: 4.2, End: 5, Text: "   "},
		{Start: 3725.87, End: 3730.1, Text: "я прошёл ocarina of time"},
	}}
	want := "[00:00:00] привет это завтракаст\n[01:02:05] я прошёл ocarina of time\n"
	if got := tr.Lines(); got != want {
		t.Errorf("lines = %q, want %q", got, want)
	}
	if got := (Transcript{}).Lines(); got != "" {
		t.Errorf("empty lines = %q", got)
	}
}
