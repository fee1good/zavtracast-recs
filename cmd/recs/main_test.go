package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fee1good/zavtracast-recs/internal/classify"
	"github.com/fee1good/zavtracast-recs/internal/feed"
	"github.com/fee1good/zavtracast-recs/internal/record"
	"github.com/fee1good/zavtracast-recs/internal/transcript"
)

const fixture = "../../internal/feed/testdata/feed.xml"

func fixtureEpisodes(t *testing.T) []feed.Episode {
	t.Helper()
	eps, err := loadFeed(t.Context(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	extra := eps[1]
	extra.Number = 396
	extra.Title = "Завтракаст 396"
	extra.AudioURL = "https://example.com/396.mp3?a=1&b=2"
	return append(eps, extra)
}

func testOptions(t *testing.T) options {
	t.Helper()
	dir := t.TempDir()
	return options{
		data:        filepath.Join(dir, "data"),
		transcripts: filepath.Join(dir, "tr"),
		min:         394,
	}
}

func writeTranscript(t *testing.T, dir string, n int) {
	t.Helper()
	tr := transcript.Transcript{
		Episode:  n,
		Model:    "tiny",
		Duration: 100,
		Segments: []transcript.Segment{{Start: 5, End: 9, Text: "поиграл в зельду"}},
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript.Path(dir, n), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeData(t *testing.T, dir string, n int) {
	t.Helper()
	if err := record.Write(dir, record.Episode{Episode: n, Title: "old"}); err != nil {
		t.Fatal(err)
	}
}

func numbers(jobs []job) []int {
	out := make([]int, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Episode)
	}
	return out
}

func TestDiscover(t *testing.T) {
	eps := fixtureEpisodes(t)
	tests := []struct {
		name    string
		min     int
		episode int
		tr      []int
		data    []int
		want    []int
		wantErr bool
	}{
		{name: "all new", min: 394, want: []int{394, 395, 396}},
		{name: "min filter", min: 395, want: []int{395, 396}},
		{
			name: "states", min: 394, tr: []int{395}, data: []int{394},
			want: []int{396},
		},
		{name: "forced with data", min: 394, episode: 394, data: []int{394}, want: []int{394}},
		{name: "forced with transcript", min: 394, episode: 395, tr: []int{395}, want: []int{}},
		{name: "forced below min", min: 400, episode: 394, want: []int{394}},
		{name: "forced unknown", min: 394, episode: 999, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := testOptions(t)
			o.min, o.episode = tt.min, tt.episode
			for _, n := range tt.tr {
				writeTranscript(t, o.transcripts, n)
			}
			for _, n := range tt.data {
				writeData(t, o.data, n)
			}
			jobs, err := discover(eps, o)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := numbers(jobs); !slices.Equal(got, tt.want) {
				t.Errorf("episodes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteMatrix(t *testing.T) {
	var buf bytes.Buffer
	if err := writeMatrix(&buf, []job{}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "matrix=[]\ncount=0\n" {
		t.Errorf("empty = %q", got)
	}
	buf.Reset()
	jobs := []job{
		{Episode: 395, AudioURL: "https://example.com/a.mp3"},
		{Episode: 396, AudioURL: "https://example.com/b.mp3?a=1&b=2"},
	}
	if err := writeMatrix(&buf, jobs); err != nil {
		t.Fatal(err)
	}
	want := `matrix=[{"episode":395,"audio_url":"https://example.com/a.mp3"},` +
		`{"episode":396,"audio_url":"https://example.com/b.mp3?a=1&b=2"}]` + "\ncount=2\n"
	if got := buf.String(); got != want {
		t.Errorf("matrix = %q, want %q", got, want)
	}
}

func TestRunDiscoverFixture(t *testing.T) {
	o := testOptions(t)
	writeData(t, o.data, 394)
	var buf bytes.Buffer
	args := []string{"discover", "-feed", fixture, "-data", o.data, "-transcripts", o.transcripts}
	if err := run(t.Context(), args, &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", lines)
	}
	m, ok := strings.CutPrefix(lines[0], "matrix=")
	if !ok {
		t.Fatalf("first line = %q", lines[0])
	}
	var jobs []job
	if err := json.Unmarshal([]byte(m), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Episode != 395 || !strings.Contains(jobs[0].AudioURL, ".mp3") {
		t.Errorf("jobs = %+v", jobs)
	}
	if lines[1] != "count=1" {
		t.Errorf("second line = %q", lines[1])
	}
}

func TestRunUnknownCommand(t *testing.T) {
	if err := run(t.Context(), []string{"nope"}, &bytes.Buffer{}); err == nil {
		t.Error("want error for unknown command")
	}
	if err := run(t.Context(), nil, &bytes.Buffer{}); err == nil {
		t.Error("want error without command")
	}
}

type genFunc func(user string) (string, error)

func (f genFunc) Generate(_ context.Context, _, user string) (string, error) {
	return f(user)
}

const goodReply = `{"records":[
{"title":"The Legend of Zelda: Ocarina of Time","title_ru":null,"type":"game",
"host":"maxim","verdict":"positive","summary":"Перепрошёл.","time":"00:00:05"},
{"title":"Roadside Picnic","title_ru":"Пикник на обочине","type":"book",
"host":"dima","verdict":"mixed","summary":"Местами затянуто.","time":"00:01:00"},
{"title":"Roadside Picnic","title_ru":null,"type":"book",
"host":"dima","verdict":"mixed","summary":"Дубль.","time":"00:02:00"}
]}`

const badReply = `{"records":[
{"title":"Good","title_ru":null,"type":"game",
"host":"maxim","verdict":"positive","summary":"Ок.","time":"00:00:05"},
{"title":"","title_ru":null,"type":"game",
"host":"maxim","verdict":"positive","summary":"Ок.","time":"00:00:05"},
{"title":"Late","title_ru":null,"type":"game",
"host":"maxim","verdict":"positive","summary":"Ок.","time":"99:00:00"}
]}`

func episodeIn(user string) int {
	for _, n := range []int{394, 395, 396} {
		if strings.Contains(user, "Завтракаст "+strconv.Itoa(n)) {
			return n
		}
	}
	return 0
}

func classifier(replies map[int]string, calls *[]int) classify.Classifier {
	return classify.Classifier{Gen: genFunc(func(user string) (string, error) {
		n := episodeIn(user)
		*calls = append(*calls, n)
		r, ok := replies[n]
		if !ok {
			return "", errors.New("boom")
		}
		return r, nil
	})}
}

func TestClassifyStates(t *testing.T) {
	eps := fixtureEpisodes(t)
	o := testOptions(t)
	writeTranscript(t, o.transcripts, 394)
	writeTranscript(t, o.transcripts, 395)
	writeData(t, o.data, 395)
	var calls []int
	cl := classifier(map[int]string{394: goodReply, 395: goodReply}, &calls)
	if err := classifyEpisodes(t.Context(), eps, o, cl); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []int{394}) {
		t.Fatalf("calls = %v, want [394]", calls)
	}
	e, err := record.Read(o.data, 394)
	if err != nil {
		t.Fatal(err)
	}
	wantDate := eps[0].Date.Format(record.DateLayout)
	if e.Duration != 15074 || e.Date != wantDate || e.PageURL != eps[0].PageURL {
		t.Errorf("episode meta = %+v", e)
	}
	ids := make([]string, 0, len(e.Records))
	for _, r := range e.Records {
		ids = append(ids, r.ID)
	}
	want := []string{
		"394-the-legend-of-zelda-ocarina-of-time-maxim",
		"394-roadside-picnic-dima",
		"394-как-американцы-нашли-русский-город-на-аляске-team",
	}
	if !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if e.Records[0].T != 5 || e.Records[1].TitleRU != "Пикник на обочине" {
		t.Errorf("records = %+v", e.Records)
	}
	old, err := record.Read(o.data, 395)
	if err != nil {
		t.Fatal(err)
	}
	if old.Title != "old" {
		t.Errorf("395 overwritten: %+v", old)
	}
}

func TestClassifyForced(t *testing.T) {
	eps := fixtureEpisodes(t)
	o := testOptions(t)
	o.episode = 395
	writeTranscript(t, o.transcripts, 394)
	writeTranscript(t, o.transcripts, 395)
	writeData(t, o.data, 395)
	var calls []int
	cl := classifier(map[int]string{394: goodReply, 395: goodReply}, &calls)
	if err := classifyEpisodes(t.Context(), eps, o, cl); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []int{395}) {
		t.Fatalf("calls = %v, want [395]", calls)
	}
	e, err := record.Read(o.data, 395)
	if err != nil {
		t.Fatal(err)
	}
	if e.Title == "old" || len(e.Records) != 2 {
		t.Errorf("395 not overwritten: %+v", e)
	}
	if record.Exists(o.data, 394) {
		t.Error("394 must not be classified")
	}
}

func TestClassifyForcedErrors(t *testing.T) {
	eps := fixtureEpisodes(t)
	var calls []int
	cl := classifier(map[int]string{395: goodReply}, &calls)

	o := testOptions(t)
	o.episode = 999
	if err := classifyEpisodes(t.Context(), eps, o, cl); err == nil {
		t.Error("want error for unknown episode")
	}

	o = testOptions(t)
	o.episode = 395
	if err := classifyEpisodes(t.Context(), eps, o, cl); err == nil {
		t.Error("want error for missing transcript")
	}
	if len(calls) != 0 {
		t.Errorf("calls = %v, want none", calls)
	}
}

func TestClassifyContinuesAfterFailure(t *testing.T) {
	eps := fixtureEpisodes(t)
	o := testOptions(t)
	writeTranscript(t, o.transcripts, 394)
	writeTranscript(t, o.transcripts, 395)
	var calls []int
	cl := classifier(map[int]string{395: goodReply}, &calls)
	err := classifyEpisodes(t.Context(), eps, o, cl)
	if err == nil || !strings.Contains(err.Error(), "episode 394") {
		t.Fatalf("err = %v, want episode 394 failure", err)
	}
	if !slices.Equal(calls, []int{394, 395}) {
		t.Errorf("calls = %v", calls)
	}
	if record.Exists(o.data, 394) {
		t.Error("394 must not be written")
	}
	if !record.Exists(o.data, 395) {
		t.Error("395 must be written")
	}
}

func TestClassifyThresholdSkip(t *testing.T) {
	eps := fixtureEpisodes(t)
	o := testOptions(t)
	writeTranscript(t, o.transcripts, 395)
	var calls []int
	cl := classifier(map[int]string{395: badReply}, &calls)
	err := classifyEpisodes(t.Context(), eps, o, cl)
	if !errors.Is(err, record.ErrTooManyInvalid) {
		t.Fatalf("err = %v, want ErrTooManyInvalid", err)
	}
	if record.Exists(o.data, 395) {
		t.Error("395 must not be written")
	}
}

func TestEvaluate(t *testing.T) {
	dir := t.TempDir()
	golden, data := filepath.Join(dir, "golden"), filepath.Join(dir, "data")
	rec := func(title, host, verdict string) record.Record {
		return record.Record{Title: title, Host: host, Verdict: verdict, Type: "game"}
	}
	g := record.Episode{Episode: 394, Records: []record.Record{
		rec("Zelda", "maxim", "positive"),
		rec("Halo", "dima", "positive"),
		rec("Doom", "timur", "negative"),
	}}
	got := record.Episode{Episode: 394, Records: []record.Record{
		rec("Zelda", "maxim", "positive"),
		rec("Doom", "timur", "mixed"),
		rec("Tetris", "dima", "positive"),
	}}
	for _, w := range []struct {
		dir string
		e   record.Episode
	}{{golden, g}, {data, got}, {golden, record.Episode{Episode: 395}}} {
		if err := record.Write(w.dir, w.e); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := evaluate(&buf, golden, data); err != nil {
		t.Fatal(err)
	}
	want := "episode 394: found 2 missed 1 extra 1 verdict 1\n" +
		"  missed halo|dima\n" +
		"  extra tetris|dima\n" +
		"total: found 2 missed 1 extra 1 verdict 1 precision 0.67 recall 0.67\n"
	if got := buf.String(); got != want {
		t.Errorf("output =\n%s\nwant\n%s", got, want)
	}
}

func TestClassifyDurationFromTranscript(t *testing.T) {
	eps := fixtureEpisodes(t)
	eps[1].Duration = 0
	o := testOptions(t)
	o.episode = 395
	tr := transcript.Transcript{Episode: 395, Model: "tiny", Duration: 99.2}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(o.transcripts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript.Path(o.transcripts, 395), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var calls []int
	cl := classifier(map[int]string{395: goodReply}, &calls)
	if err := classifyEpisodes(t.Context(), eps, o, cl); err != nil {
		t.Fatal(err)
	}
	e, err := record.Read(o.data, 395)
	if err != nil {
		t.Fatal(err)
	}
	if e.Duration != 100 || len(e.Records) != 2 {
		t.Errorf("duration = %d, records = %d, want 100 and 2", e.Duration, len(e.Records))
	}
}

func TestClassifyCancelled(t *testing.T) {
	eps := fixtureEpisodes(t)
	o := testOptions(t)
	writeTranscript(t, o.transcripts, 394)
	writeTranscript(t, o.transcripts, 395)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var calls []int
	cl := classifier(map[int]string{394: goodReply, 395: goodReply}, &calls)
	if err := classifyEpisodes(ctx, eps, o, cl); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(calls) != 0 {
		t.Errorf("calls = %v, want none", calls)
	}
}

func TestRunClassifyWithoutKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	err := run(t.Context(), []string{"classify", "-feed", "/nonexistent.xml"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("err = %v, want missing key error", err)
	}
}

func TestRunBuildRealSite(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	out := filepath.Join(dir, "out")
	writeData(t, data, 395)
	args := []string{"build", "-site", "../../site", "-data", data, "-out", out,
		"-goatcounter", "abc"}
	if err := run(t.Context(), args, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `data-goatcounter="https://abc.goatcounter.com/count"`) {
		t.Error("goatcounter snippet missing from index.html")
	}
	for _, name := range []string{"app.js", "style.css", "data.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Error(err)
		}
	}
}
