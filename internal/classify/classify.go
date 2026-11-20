package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/fee1good/zavtracast-recs/internal/clock"
	"github.com/fee1good/zavtracast-recs/internal/feed"
	"github.com/fee1good/zavtracast-recs/internal/record"
	"github.com/fee1good/zavtracast-recs/internal/transcript"
)

type Generator interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

const SystemPrompt = `Ты разбираешь транскрипт выпуска подкаста «Завтракаст» и извлекаешь ` +
	`рекомендации ведущих.

Что считать записью:
- Запись — это личный опыт ведущего или гостя с произведением: он сам поиграл, посмотрел ` +
	`или прочитал его и высказал своё мнение.
- Не создавай записи для новостей, слухов, анонсов, трейлеров и пересказов чужих ` +
	`обзоров и оценок.
- Одна запись на произведение и ведущего. Если два ведущих высказались о ` +
	`произведении по-разному, это две записи.

Поля записи:
- host: dima (Дима), maxim (Максим, Макс), timur (Тимур), guest:<имя> для гостя ` +
	`или unknown, если не уверен, кто говорит.
- title: название на языке оригинала, как оно официально пишется.
- title_ru: официальное русское название, только если оно существует и отличается ` +
	`от title, иначе null.
- type: тип произведения.
- verdict: positive, mixed или negative по итоговому мнению ведущего.
- summary: пересказ мнения своими словами на русском, 1-2 предложения, без цитат.
- time: метка [HH:MM:SS] строки транскрипта, с которой начинается обсуждение ` +
	`произведения, в формате HH:MM:SS.

Шоуноты и чек-лист впечатлений:
- Список «Впечатления из шоунотов» — это чек-лист. Найди каждый пункт в транскрипте ` +
	`и используй пункт для правильного написания названий и определения ведущего.
- Пункт, от которого нет следа в транскрипте, не превращается в запись.
- Источник истины — транскрипт. В нём нет пунктуации, и иностранные названия ` +
	`и имена могут быть записаны с ошибками.`

type Reply struct {
	Records []ReplyRecord `json:"records"`
}

type ReplyRecord struct {
	Title   string `json:"title"`
	TitleRU string `json:"title_ru"`
	Type    string `json:"type"`
	Host    string `json:"host"`
	Verdict string `json:"verdict"`
	Summary string `json:"summary"`
	Time    string `json:"time"`
}

var DefaultBackoff = []time.Duration{10 * time.Second, 30 * time.Second, 90 * time.Second}

func UserPrompt(ep feed.Episode, tr transcript.Transcript) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Выпуск\n%s\n\n", ep.Title)
	b.WriteString("## Впечатления из шоунотов\n")
	writeList(&b, ep.Impressions)
	b.WriteString("\n## Шоуноты\n")
	writeList(&b, ep.Shownotes)
	b.WriteString("\n## Транскрипт\n")
	b.WriteString(tr.Lines())
	return b.String()
}

func writeList(b *strings.Builder, items []string) {
	if len(items) == 0 {
		b.WriteString("нет\n")
		return
	}
	for i, it := range items {
		fmt.Fprintf(b, "%d. %s\n", i+1, it)
	}
}

func Schema() *genai.Schema {
	str := func() *genai.Schema { return &genai.Schema{Type: genai.TypeString} }
	fields := []string{"title", "title_ru", "type", "host", "verdict", "summary", "time"}
	rec := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"title":    str(),
			"title_ru": {Type: genai.TypeString, Nullable: new(true)},
			"type": {
				Type: genai.TypeString, Format: "enum", Enum: record.MediaTypes,
			},
			"host": {
				Type:        genai.TypeString,
				Description: "dima, maxim, timur, guest:<имя> или unknown",
			},
			"verdict": {
				Type: genai.TypeString, Format: "enum", Enum: record.Verdicts,
			},
			"summary": str(),
			"time":    {Type: genai.TypeString, Description: "HH:MM:SS"},
		},
		Required:         fields,
		PropertyOrdering: fields,
	}
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"records": {Type: genai.TypeArray, Items: rec},
		},
		Required: []string{"records"},
	}
}

type Classifier struct {
	Gen     Generator
	Backoff []time.Duration
}

func (c Classifier) Classify(
	ctx context.Context, ep feed.Episode, tr transcript.Transcript,
) ([]record.Record, error) {
	user := UserPrompt(ep, tr)
	var lastErr error
	for attempt := range len(c.Backoff) + 1 {
		if attempt > 0 {
			if err := wait(ctx, c.Backoff[attempt-1]); err != nil {
				return nil, fmt.Errorf("classify episode %d: %w", ep.Number, err)
			}
		}
		recs, err := c.once(ctx, user)
		if err == nil {
			return recs, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("classify episode %d: %w", ep.Number, lastErr)
}

func (c Classifier) once(ctx context.Context, user string) ([]record.Record, error) {
	text, err := c.Gen.Generate(ctx, SystemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	var r Reply
	if err := json.Unmarshal([]byte(text), &r); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}
	recs := make([]record.Record, 0, len(r.Records))
	for _, rr := range r.Records {
		recs = append(recs, rr.toRecord())
	}
	return recs, nil
}

func (rr ReplyRecord) toRecord() record.Record {
	t, err := clock.Parse(strings.Trim(rr.Time, "[] \t\n"))
	if err != nil {
		t = -1
	}
	title := strings.TrimSpace(rr.Title)
	titleRU := strings.TrimSpace(rr.TitleRU)
	if titleRU == title {
		titleRU = ""
	}
	return record.Record{
		Title:   title,
		TitleRU: titleRU,
		Type:    strings.TrimSpace(rr.Type),
		Host:    strings.TrimSpace(rr.Host),
		Verdict: strings.TrimSpace(rr.Verdict),
		Summary: strings.TrimSpace(rr.Summary),
		T:       t,
	}
}

func wait(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func Articles(ep feed.Episode) []record.Record {
	recs := make([]record.Record, 0, len(ep.Articles))
	for _, a := range ep.Articles {
		recs = append(recs, record.Record{
			Title:   a.Title,
			Type:    record.TypeArticle,
			Host:    record.HostTeam,
			Verdict: "positive",
			URL:     a.URL,
		})
	}
	return recs
}
