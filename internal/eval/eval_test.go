package eval

import (
	"reflect"
	"testing"

	"github.com/fee1good/zavtracast-recs/internal/record"
)

func rec(title, host, verdict string) record.Record {
	return record.Record{Title: title, Type: "game", Host: host, Verdict: verdict}
}

func article(title string) record.Record {
	return record.Record{
		Title: title, Type: record.TypeArticle, Host: record.HostTeam,
		Verdict: "positive", URL: "https://example.com",
	}
}

func TestCompare(t *testing.T) {
	golden := []record.Record{
		rec("The Legend of Zelda: Ocarina of Time", "maxim", "positive"),
		rec("Пикник на обочине", "dima", "mixed"),
		rec("Hades", "timur", "positive"),
		rec("Hades", "timur", "negative"),
		article("Golden article"),
	}
	got := []record.Record{
		rec("The Legend of Zelda Ocarina of Time", "maxim", "positive"),
		rec("пикник на обочине", "dima", "negative"),
		rec("Hades", "dima", "positive"),
		article("Got article"),
	}
	want := Result{
		Found:           []string{"the-legend-of-zelda-ocarina-of-time|maxim", "пикник-на-обочине|dima"},
		Missed:          []string{"hades|timur"},
		Extra:           []string{"hades|dima"},
		VerdictMismatch: []string{"пикник-на-обочине|dima"},
	}
	res := Compare(golden, got)
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("compare:\n got %+v\nwant %+v", res, want)
	}
	if p := res.Precision(); p != 2.0/3 {
		t.Errorf("precision = %v", p)
	}
	if r := res.Recall(); r != 2.0/3 {
		t.Errorf("recall = %v", r)
	}
}

func TestCompareArticlesOnly(t *testing.T) {
	res := Compare([]record.Record{article("a")}, []record.Record{article("b")})
	if !reflect.DeepEqual(res, Result{}) {
		t.Errorf("result = %+v, want empty", res)
	}
	if res.Precision() != 0 || res.Recall() != 0 {
		t.Errorf("precision %v recall %v, want 0", res.Precision(), res.Recall())
	}
}

func TestPrecisionRecall(t *testing.T) {
	tests := []struct {
		name       string
		res        Result
		prec, recl float64
	}{
		{"perfect", Result{Found: []string{"a", "b"}}, 1, 1},
		{"only extra", Result{Extra: []string{"a"}}, 0, 0},
		{"only missed", Result{Missed: []string{"a"}}, 0, 0},
		{"mixed", Result{Found: []string{"a"}, Extra: []string{"b", "c", "d"}, Missed: []string{"e"}}, 0.25, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.res.Precision(); got != tt.prec {
				t.Errorf("precision = %v, want %v", got, tt.prec)
			}
			if got := tt.res.Recall(); got != tt.recl {
				t.Errorf("recall = %v, want %v", got, tt.recl)
			}
		})
	}
}
