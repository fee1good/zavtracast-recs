package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/fee1good/zavtracast-recs/internal/classify"
	"github.com/fee1good/zavtracast-recs/internal/feed"
	"github.com/fee1good/zavtracast-recs/internal/record"
	"github.com/fee1good/zavtracast-recs/internal/site"
)

type options struct {
	feed        string
	data        string
	transcripts string
	min         int
	episode     int
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	err := run(context.Background(), os.Args[1:], os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		slog.Error("recs failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: recs discover|classify|build|eval [flags]")
	}
	name, rest := args[0], args[1:]
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var o options
	fs.StringVar(&o.feed, "feed", feed.URL, "feed url or local file")
	fs.StringVar(&o.data, "data", "data/episodes", "episode data dir")
	fs.StringVar(&o.transcripts, "transcripts", ".transcripts", "transcripts dir")
	fs.IntVar(&o.min, "min", 386, "first episode number")
	fs.IntVar(&o.episode, "episode", 0, "process only this episode")
	switch name {
	case "discover":
		if err := fs.Parse(rest); err != nil {
			return err
		}
		eps, err := loadFeed(ctx, o.feed)
		if err != nil {
			return err
		}
		jobs, err := discover(eps, o)
		if err != nil {
			return err
		}
		return writeMatrix(stdout, jobs)
	case "classify":
		model := fs.String("model",
			cmp.Or(os.Getenv("RECS_GEMINI_MODEL"), classify.DefaultModel), "gemini model")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return errors.New("GEMINI_API_KEY is not set")
		}
		eps, err := loadFeed(ctx, o.feed)
		if err != nil {
			return err
		}
		gen, err := classify.NewGemini(ctx, key, *model)
		if err != nil {
			return err
		}
		cl := classify.Classifier{Gen: gen, Backoff: classify.DefaultBackoff}
		return classifyEpisodes(ctx, eps, o, cl)
	case "build":
		siteDir := fs.String("site", "site", "site sources dir")
		out := fs.String("out", "_site", "output dir")
		gc := fs.String("goatcounter", "", "goatcounter code")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		eps, err := record.ReadAll(o.data)
		if err != nil {
			return err
		}
		if err := site.Build(*siteDir, *out, eps, *gc); err != nil {
			return err
		}
		slog.Info("site built", "out", *out, "episodes", len(eps))
		return nil
	case "eval":
		golden := fs.String("golden", "eval/golden", "golden episodes dir")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		return evaluate(stdout, *golden, o.data)
	default:
		return fmt.Errorf("unknown command %q", name)
	}
}

func loadFeed(ctx context.Context, src string) ([]feed.Episode, error) {
	if strings.HasPrefix(src, "http") {
		return feed.Fetch(ctx, src)
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open feed: %w", err)
	}
	defer f.Close()
	return feed.Parse(f)
}
