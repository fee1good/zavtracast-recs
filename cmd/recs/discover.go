package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/fee1good/zavtracast-recs/internal/feed"
	"github.com/fee1good/zavtracast-recs/internal/record"
	"github.com/fee1good/zavtracast-recs/internal/transcript"
)

type job struct {
	Episode  int    `json:"episode"`
	AudioURL string `json:"audio_url"`
}

func candidates(eps []feed.Episode, o options) ([]feed.Episode, error) {
	if o.episode != 0 {
		i := slices.IndexFunc(eps, func(e feed.Episode) bool { return e.Number == o.episode })
		if i < 0 {
			return nil, fmt.Errorf("episode %d not found in feed", o.episode)
		}
		return []feed.Episode{eps[i]}, nil
	}
	var out []feed.Episode
	for _, e := range eps {
		if e.Number >= o.min && !record.Exists(o.data, e.Number) {
			out = append(out, e)
		}
	}
	return out, nil
}

func discover(eps []feed.Episode, o options) ([]job, error) {
	cands, err := candidates(eps, o)
	if err != nil {
		return nil, err
	}
	jobs := []job{}
	for _, e := range cands {
		if !transcript.Exists(o.transcripts, e.Number) {
			jobs = append(jobs, job{Episode: e.Number, AudioURL: e.AudioURL})
		}
	}
	return jobs, nil
}

func writeMatrix(w io.Writer, jobs []job) error {
	if jobs == nil {
		jobs = []job{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(jobs); err != nil {
		return fmt.Errorf("encode matrix: %w", err)
	}
	_, err := fmt.Fprintf(w, "matrix=%s\ncount=%d\n", bytes.TrimSpace(buf.Bytes()), len(jobs))
	return err
}
