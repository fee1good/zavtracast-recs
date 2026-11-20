package classify

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"github.com/fee1good/zavtracast-recs/internal/feed"
	"github.com/fee1good/zavtracast-recs/internal/record"
	"github.com/fee1good/zavtracast-recs/internal/transcript"
)

type step struct {
	text string
	err  error
}

type fakeGen struct {
	steps  []step
	calls  int
	system string
	user   string
}

func (f *fakeGen) Generate(_ context.Context, system, user string) (string, error) {
	f.system, f.user = system, user
	s := f.steps[min(f.calls, len(f.steps)-1)]
	f.calls++
	return s.text, s.err
}

const happyReply = `{"records":[
{"title":"The Legend of Zelda: Ocarina of Time","title_ru":null,"type":"game",
"host":"maxim","verdict":"positive","summary":"Перепрошёл на эмуляторе.","time":"01:30:12"},
{"title":"Roadside Picnic","title_ru":"Пикник на обочине","type":"book",
"host":"dima","verdict":"mixed","summary":"Местами затянуто.","time":"[0:05]"},
{"title":"Bad Time","title_ru":"Bad Time","type":"movie",
"host":"timur","verdict":"negative","summary":"Скучно.","time":"когда-то"}
]}`

func sampleEpisode() feed.Episode {
	return feed.Episode{
		Number:      395,
		Title:       "Завтракаст 395 – Nvidiot",
		Impressions: []string{"Максим поиграл в Zelda", "Дима прочитал Пикник"},
		Shownotes:   []string{"Новость про Nvidia", "Максим поиграл в Zelda"},
		Articles: []feed.Article{
			{Title: "Как американцы нашли русский город", URL: "https://youtu.be/x"},
		},
	}
}

func sampleTranscript() transcript.Transcript {
	return transcript.Transcript{
		Episode: 395,
		Segments: []transcript.Segment{
			{Start: 5412.4, End: 5415, Text: " я тут перепрошел зельду "},
		},
	}
}

func TestClassifyHappyPath(t *testing.T) {
	gen := &fakeGen{steps: []step{{text: happyReply}}}
	c := Classifier{Gen: gen}
	got, err := c.Classify(t.Context(), sampleEpisode(), sampleTranscript())
	if err != nil {
		t.Fatal(err)
	}
	want := []record.Record{
		{
			Title: "The Legend of Zelda: Ocarina of Time", Type: "game", Host: "maxim",
			Verdict: "positive", Summary: "Перепрошёл на эмуляторе.", T: 5412,
		},
		{
			Title: "Roadside Picnic", TitleRU: "Пикник на обочине", Type: "book",
			Host: "dima", Verdict: "mixed", Summary: "Местами затянуто.", T: 5,
		},
		{
			Title: "Bad Time", Type: "movie", Host: "timur", Verdict: "negative",
			Summary: "Скучно.", T: -1,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if gen.calls != 1 {
		t.Fatalf("calls = %d, want 1", gen.calls)
	}
	if gen.system != SystemPrompt {
		t.Fatal("system prompt not passed")
	}
	if !strings.Contains(gen.user, "[01:30:12] я тут перепрошел зельду") {
		t.Fatalf("user prompt lacks transcript:\n%s", gen.user)
	}
}

func TestClassifyRetries(t *testing.T) {
	boom := errors.New("boom")
	zero := []time.Duration{0, 0, 0}
	tests := []struct {
		name      string
		steps     []step
		wantCalls int
		wantErr   bool
	}{
		{
			name:      "error twice then success",
			steps:     []step{{err: boom}, {err: boom}, {text: `{"records":[]}`}},
			wantCalls: 3,
		},
		{
			name:      "always error",
			steps:     []step{{err: boom}},
			wantCalls: len(zero) + 1,
			wantErr:   true,
		},
		{
			name:      "invalid json retried",
			steps:     []step{{text: "not json"}, {text: `{"records":[]}`}},
			wantCalls: 2,
		},
		{
			name:      "always invalid json",
			steps:     []step{{text: "{"}},
			wantCalls: len(zero) + 1,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &fakeGen{steps: tt.steps}
			c := Classifier{Gen: gen, Backoff: zero}
			got, err := c.Classify(t.Context(), sampleEpisode(), sampleTranscript())
			if gen.calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", gen.calls, tt.wantCalls)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				if !strings.Contains(err.Error(), "episode 395") {
					t.Fatalf("error lacks episode: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || len(got) != 0 {
				t.Fatalf("got %v, want empty non-nil", got)
			}
		})
	}
}

func TestClassifyLastErrorWrapped(t *testing.T) {
	boom := errors.New("boom")
	gen := &fakeGen{steps: []step{{err: boom}}}
	c := Classifier{Gen: gen}
	_, err := c.Classify(t.Context(), sampleEpisode(), sampleTranscript())
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped boom", err)
	}
	if gen.calls != 1 {
		t.Fatalf("calls = %d, want 1", gen.calls)
	}
}

func TestClassifyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	gen := &fakeGen{steps: []step{{err: errors.New("boom")}}}
	c := Classifier{Gen: gen, Backoff: []time.Duration{time.Hour, time.Hour}}
	start := time.Now()
	_, err := c.Classify(ctx, sampleEpisode(), sampleTranscript())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
	if gen.calls != 1 {
		t.Fatalf("calls = %d, want 1", gen.calls)
	}
	if time.Since(start) > time.Second {
		t.Fatal("retries did not stop on cancelled context")
	}
}

func TestClassifyCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	gen := &fakeGen{steps: []step{{err: errors.New("boom")}}}
	c := Classifier{Gen: gen, Backoff: []time.Duration{time.Hour}}
	_, err := c.Classify(ctx, sampleEpisode(), sampleTranscript())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if gen.calls != 1 {
		t.Fatalf("calls = %d, want 1", gen.calls)
	}
}

func TestUserPrompt(t *testing.T) {
	got := UserPrompt(sampleEpisode(), sampleTranscript())
	for _, want := range []string{
		"## Выпуск\nЗавтракаст 395 – Nvidiot\n",
		"## Впечатления из шоунотов\n1. Максим поиграл в Zelda\n2. Дима прочитал Пикник\n",
		"## Шоуноты\n1. Новость про Nvidia\n2. Максим поиграл в Zelda\n",
		"## Транскрипт\n[01:30:12] я тут перепрошел зельду\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt lacks %q:\n%s", want, got)
		}
	}
	order := []string{"## Выпуск", "## Впечатления", "## Шоуноты", "## Транскрипт"}
	idx := make([]int, len(order))
	for i, h := range order {
		idx[i] = strings.Index(got, h)
	}
	if !slices.IsSorted(idx) {
		t.Errorf("sections out of order: %v", idx)
	}
}

func TestUserPromptEmptyLists(t *testing.T) {
	got := UserPrompt(feed.Episode{Title: "Завтракаст 1"}, transcript.Transcript{})
	if !strings.Contains(got, "## Впечатления из шоунотов\nнет\n") {
		t.Fatalf("empty impressions not marked:\n%s", got)
	}
}

func TestArticles(t *testing.T) {
	got := Articles(sampleEpisode())
	want := []record.Record{{
		Title:   "Как американцы нашли русский город",
		Type:    record.TypeArticle,
		Host:    record.HostTeam,
		Verdict: "positive",
		URL:     "https://youtu.be/x",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	cleaned, dropped := record.Clean(395, 100, got)
	if len(dropped) != 0 || len(cleaned) != 1 {
		t.Fatalf("article record invalid: %v", dropped)
	}
}

func TestSchema(t *testing.T) {
	s := Schema()
	if !slices.Equal(s.Required, []string{"records"}) {
		t.Fatalf("top required = %v", s.Required)
	}
	rec := s.Properties["records"].Items
	if rec == nil || rec.Type != genai.TypeObject {
		t.Fatal("records items not an object")
	}
	for name := range rec.Properties {
		if !slices.Contains(rec.Required, name) {
			t.Errorf("field %s not required", name)
		}
	}
	if len(rec.Properties) != 7 {
		t.Errorf("fields = %d, want 7", len(rec.Properties))
	}
	if !slices.Equal(rec.Properties["type"].Enum, record.MediaTypes) {
		t.Errorf("type enum = %v", rec.Properties["type"].Enum)
	}
	if !slices.Equal(rec.Properties["verdict"].Enum, record.Verdicts) {
		t.Errorf("verdict enum = %v", rec.Properties["verdict"].Enum)
	}
	if n := rec.Properties["title_ru"].Nullable; n == nil || !*n {
		t.Error("title_ru not nullable")
	}
}

func TestReplyText(t *testing.T) {
	cand := func(text string, reason genai.FinishReason) *genai.Candidate {
		return &genai.Candidate{
			Content:      genai.NewContentFromText(text, genai.RoleModel),
			FinishReason: reason,
		}
	}
	tests := map[string]struct {
		resp    *genai.GenerateContentResponse
		want    string
		wantErr string
	}{
		"ok": {
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{cand(`{"records":[]}`, genai.FinishReasonStop)},
			},
			want: `{"records":[]}`,
		},
		"no candidates": {
			resp:    &genai.GenerateContentResponse{},
			wantErr: "empty reply: finish_reason=none",
		},
		"blocked": {
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{{FinishReason: genai.FinishReasonSafety}},
				PromptFeedback: &genai.GenerateContentResponsePromptFeedback{
					BlockReason: genai.BlockedReasonSafety,
				},
			},
			wantErr: "empty reply: finish_reason=SAFETY block_reason=SAFETY",
		},
		"truncated": {
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{cand(`{"records":[{"ti`, genai.FinishReasonMaxTokens)},
			},
			wantErr: "truncated reply: finish_reason=MAX_TOKENS",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := replyText(tt.resp)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
