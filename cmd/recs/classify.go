package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/fee1good/zavtracast-recs/internal/classify"
	"github.com/fee1good/zavtracast-recs/internal/feed"
	"github.com/fee1good/zavtracast-recs/internal/record"
	"github.com/fee1good/zavtracast-recs/internal/transcript"
)

func classifyTargets(eps []feed.Episode, o options) ([]feed.Episode, error) {
	if o.episode != 0 {
		return candidates(eps, o)
	}
	var out []feed.Episode
	for _, e := range eps {
		if e.Number >= o.min && transcript.Exists(o.transcripts, e.Number) &&
			!record.Exists(o.data, e.Number) {
			out = append(out, e)
		}
	}
	return out, nil
}

func classifyEpisodes(
	ctx context.Context, eps []feed.Episode, o options, cl classify.Classifier,
) error {
	targets, err := classifyTargets(eps, o)
	if err != nil {
		return err
	}
	slog.Info("classify targets", "count", len(targets))
	var errs []error
	for _, e := range targets {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := classifyEpisode(ctx, e, o, cl); err != nil {
			slog.Error("episode failed", "episode", e.Number, "err", err)
			errs = append(errs, fmt.Errorf("episode %d: %w", e.Number, err))
		}
	}
	return errors.Join(errs...)
}

func classifyEpisode(
	ctx context.Context, e feed.Episode, o options, cl classify.Classifier,
) error {
	tr, err := transcript.Load(o.transcripts, e.Number)
	if err != nil {
		return err
	}
	duration := cmp.Or(e.Duration, int(math.Ceil(tr.Duration)))
	recs, err := cl.Classify(ctx, e, tr)
	if err != nil {
		return err
	}
	kept, dropped := record.Clean(e.Number, duration, recs)
	logDropped(e.Number, dropped)
	if err := record.CheckDropped(len(recs), len(dropped)); err != nil {
		return fmt.Errorf("%d of %d dropped: %w", len(dropped), len(recs), err)
	}
	arts, artDropped := record.Clean(e.Number, duration, classify.Articles(e))
	logDropped(e.Number, artDropped)
	out := record.Episode{
		Episode:  e.Number,
		Title:    e.Title,
		Date:     e.Date.Format(record.DateLayout),
		PageURL:  e.PageURL,
		AudioURL: e.AudioURL,
		Duration: duration,
		Records:  append(kept, arts...),
	}
	if err := record.Write(o.data, out); err != nil {
		return err
	}
	slog.Info("episode written", "episode", e.Number, "records", len(kept), "articles", len(arts))
	return nil
}

func logDropped(ep int, dropped []error) {
	for _, err := range dropped {
		slog.Warn("record dropped", "episode", ep, "err", err)
	}
}
