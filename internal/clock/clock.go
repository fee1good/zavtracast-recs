package clock

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func Format(sec int) string {
	return fmt.Sprintf("%02d:%02d:%02d", sec/3600, sec%3600/60, sec%60)
}

func Parse(s string) (int, error) {
	var parts []int
	for p := range strings.SplitSeq(s, ":") {
		if p == "" {
			return 0, fmt.Errorf("empty part in %q", s)
		}
		if strings.ContainsFunc(p, func(r rune) bool { return r < '0' || r > '9' }) {
			return 0, fmt.Errorf("invalid part %q in %q", p, s)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, fmt.Errorf("parse %q: %w", s, err)
		}
		parts = append(parts, n)
	}
	if len(parts) > 3 {
		return 0, errors.New("too many parts in " + strconv.Quote(s))
	}
	total := 0
	for i, n := range parts {
		if i > 0 && n >= 60 {
			return 0, fmt.Errorf("part %d out of range in %q", n, s)
		}
		total = total*60 + n
	}
	return total, nil
}
