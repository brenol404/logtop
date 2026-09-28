package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brenol404/logtop/parser"
)

func entries() []parser.Entry {
	return []parser.Entry{
		{Path: "/a", Status: 200, LatencyMs: 10, HasLatency: true},
		{Path: "/a", Status: 500, LatencyMs: 30, HasLatency: true},
		{Path: "/b", Status: 200, LatencyMs: 20, HasLatency: true},
		{Path: "/b", Status: 200},
	}
}

func TestAggregate(t *testing.T) {
	s := Aggregate(entries())
	if s.Total != 4 || s.Skipped != 0 {
		t.Fatalf("totals: %+v", s)
	}
	if s.Status[200] != 3 || s.Status[500] != 1 {
		t.Fatalf("status: %v", s.Status)
	}
	if s.Paths["/a"].Hits != 2 || s.Paths["/a"].Err5xx != 1 {
		t.Fatalf("path a: %+v", s.Paths["/a"])
	}
	top := s.TopPaths(1)
	if len(top) != 1 || (top[0].Path != "/a" && top[0].Path != "/b") {
		t.Fatalf("top: %+v", top) // tie 2-2, deterministic by path
	}
	if got := Percentile([]float64{1, 2, 3, 4}, 50); got != 2 {
		t.Fatalf("p50: %v", got)
	}
	if got := Percentile(nil, 99); got != 0 {
		t.Fatalf("empty: %v", got)
	}
}

func TestRunMatchesSequential(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 2500; i++ {
		fmt.Fprintf(&sb, "10.0.0.1 - - [11/Oct/2000:09:00:00 +0000] \"GET /p%d HTTP/1.1\" 200 100 %d\n", i%50, 1000+i)
	}
	sb.WriteString("garbage line\n")
	want, err := Run(strings.NewReader(sb.String()), 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Run(strings.NewReader(sb.String()), 4)
	if err != nil {
		t.Fatal(err)
	}
	if want.Total != got.Total || want.Skipped != got.Skipped {
		t.Fatalf("totals differ: %+v vs %+v", want.Total, got.Total)
	}
	if want.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", want.Skipped)
	}
	a, b := want.TopPaths(0), got.TopPaths(0)
	if len(a) != len(b) {
		t.Fatalf("top len: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].Hits != b[i].Hits || a[i].Err5xx != b[i].Err5xx {
			t.Fatalf("row %d differs: %+v vs %+v", i, a[i], b[i])
		}
		if len(a[i].Lat) != len(b[i].Lat) {
			t.Fatalf("row %d lat len differs", i)
		}
	}
	if Percentile(want.LatAll, 50) != Percentile(got.LatAll, 50) {
		t.Fatalf("p50 differs")
	}
}
