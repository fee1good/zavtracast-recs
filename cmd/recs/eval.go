package main

import (
	"fmt"
	"io"

	"github.com/fee1good/zavtracast-recs/internal/eval"
	"github.com/fee1good/zavtracast-recs/internal/record"
)

func evaluate(w io.Writer, goldenDir, dataDir string) error {
	golden, err := record.ReadAll(goldenDir)
	if err != nil {
		return err
	}
	var total eval.Result
	for _, g := range golden {
		if !record.Exists(dataDir, g.Episode) {
			continue
		}
		got, err := record.Read(dataDir, g.Episode)
		if err != nil {
			return err
		}
		res := eval.Compare(g.Records, got.Records)
		fmt.Fprintf(w, "episode %d: found %d missed %d extra %d verdict %d\n",
			g.Episode, len(res.Found), len(res.Missed), len(res.Extra),
			len(res.VerdictMismatch))
		for _, k := range res.Missed {
			fmt.Fprintf(w, "  missed %s\n", k)
		}
		for _, k := range res.Extra {
			fmt.Fprintf(w, "  extra %s\n", k)
		}
		total.Found = append(total.Found, res.Found...)
		total.Missed = append(total.Missed, res.Missed...)
		total.Extra = append(total.Extra, res.Extra...)
		total.VerdictMismatch = append(total.VerdictMismatch, res.VerdictMismatch...)
	}
	_, err = fmt.Fprintf(w,
		"total: found %d missed %d extra %d verdict %d precision %.2f recall %.2f\n",
		len(total.Found), len(total.Missed), len(total.Extra), len(total.VerdictMismatch),
		total.Precision(), total.Recall())
	return err
}
