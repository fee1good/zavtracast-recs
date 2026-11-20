package record

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const golden = `{
  "episode": 395,
  "title": "Завтракаст 395 – Nvidiot & <co>",
  "date": "2026-11-11",
  "page_url": "https://zavtracast.ru/395.html",
  "audio_url": "https://media.example.com/395.mp3?a=1&b=2",
  "duration": 8261,
  "records": [
    {
      "id": "395-the-legend-of-zelda-ocarina-of-time-maxim",
      "title": "The Legend of Zelda: Ocarina of Time",
      "type": "game",
      "host": "maxim",
      "verdict": "positive",
      "summary": "Перепрошёл на эмуляторе",
      "t": 5412
    },
    {
      "id": "395-пикник-на-обочине-dima",
      "title": "Roadside Picnic",
      "title_ru": "Пикник на обочине",
      "type": "book",
      "host": "dima",
      "verdict": "mixed",
      "summary": "Местами затянуто",
      "t": 0
    },
    {
      "id": "395-статья-team",
      "title": "Статья",
      "type": "article",
      "host": "team",
      "verdict": "positive",
      "url": "https://example.com/a?x=1&y=2",
      "t": 0
    }
  ]
}
`

func sampleEpisode() Episode {
	return Episode{
		Episode:  395,
		Title:    "Завтракаст 395 – Nvidiot & <co>",
		Date:     "2026-11-11",
		PageURL:  "https://zavtracast.ru/395.html",
		AudioURL: "https://media.example.com/395.mp3?a=1&b=2",
		Duration: 8261,
		Records: []Record{
			{
				ID:      "395-the-legend-of-zelda-ocarina-of-time-maxim",
				Title:   "The Legend of Zelda: Ocarina of Time",
				Type:    "game",
				Host:    "maxim",
				Verdict: "positive",
				Summary: "Перепрошёл на эмуляторе",
				T:       5412,
			},
			{
				ID:      "395-пикник-на-обочине-dima",
				Title:   "Roadside Picnic",
				TitleRU: "Пикник на обочине",
				Type:    "book",
				Host:    "dima",
				Verdict: "mixed",
				Summary: "Местами затянуто",
			},
			{
				ID:      "395-статья-team",
				Title:   "Статья",
				Type:    TypeArticle,
				Host:    HostTeam,
				Verdict: "positive",
				URL:     "https://example.com/a?x=1&y=2",
			},
		},
	}
}

func TestWriteGolden(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data", "episodes")
	if err := Write(dir, sampleEpisode()); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, err := os.ReadFile(Path(dir, 395))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Errorf("bytes mismatch:\n%s\nwant:\n%s", b, golden)
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleEpisode()
	if Exists(dir, 395) {
		t.Fatal("exists before write")
	}
	if err := Write(dir, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !Exists(dir, 395) {
		t.Fatal("not exists after write")
	}
	got, err := Read(dir, 395)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, want)
	}
}

func TestWriteEmptyRecords(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Episode{Episode: 7, Date: "2026-01-02"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path(dir, 7))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "episode": 7,
  "title": "",
  "date": "2026-01-02",
  "page_url": "",
  "audio_url": "",
  "duration": 0,
  "records": []
}
`
	if string(b) != want {
		t.Errorf("got:\n%s\nwant:\n%s", b, want)
	}
}

func TestReadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Read(dir, 1); err == nil {
		t.Error("missing file: no error")
	}
	if err := os.WriteFile(Path(dir, 2), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, 2); err == nil {
		t.Error("bad json: no error")
	}
}

func TestReadAll(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []int{395, 7, 100} {
		if err := Write(dir, Episode{Episode: n}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.json"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "8.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	eps, err := ReadAll(dir)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	var got []int
	for _, e := range eps {
		got = append(got, e.Episode)
	}
	if want := []int{7, 100, 395}; !reflect.DeepEqual(got, want) {
		t.Errorf("episodes = %v, want %v", got, want)
	}
}

func TestReadAllMissingDir(t *testing.T) {
	eps, err := ReadAll(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(eps) != 0 {
		t.Errorf("eps = %v", eps)
	}
}

func TestReadAllBadFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir, 3), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadAll(dir); err == nil {
		t.Error("bad file: no error")
	}
}
