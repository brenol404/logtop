// Command logtop analyzes Apache/Nginx access logs concurrently:
//
//	logtop access.log --top 10 --workers 8
//	cat access.log | logtop --format json
//
// Input: Common or Combined Log Format on a file or stdin, with an optional
// trailing %D latency field (microseconds). Output: top paths, status
// histogram and latency percentiles, as text or JSON.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/brenol404/logtop/analyzer"
)

type pathRow struct {
	Path   string  `json:"path"`
	Hits   int     `json:"hits"`
	Err5xx int     `json:"err_5xx"`
	P50    float64 `json:"p50_ms,omitempty"`
	P95    float64 `json:"p95_ms,omitempty"`
}

type report struct {
	Total   int         `json:"total"`
	Skipped int         `json:"skipped"`
	Status  map[int]int `json:"status"`
	Paths   []pathRow   `json:"paths"`
	LatP50  float64     `json:"lat_p50_ms,omitempty"`
	LatP95  float64     `json:"lat_p95_ms,omitempty"`
	Workers int         `json:"workers"`
}

func main() {
	topN := flag.Int("top", 10, "show top N paths")
	workers := flag.Int("workers", 0, "worker goroutines (0 = NumCPU)")
	format := flag.String("format", "text", "output format: text or json")
	flag.Parse()

	var input io.Reader = os.Stdin
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "logtop: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		input = f
	}

	stats, err := analyzer.Run(input, *workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logtop: %v\n", err)
		os.Exit(1)
	}
	rep := report{
		Total:   stats.Total,
		Skipped: stats.Skipped,
		Status:  stats.Status,
		Workers: *workers,
	}
	if len(stats.LatAll) > 0 {
		rep.LatP50 = analyzer.Percentile(stats.LatAll, 50)
		rep.LatP95 = analyzer.Percentile(stats.LatAll, 95)
	}
	for _, p := range stats.TopPaths(*topN) {
		row := pathRow{Path: p.Path, Hits: p.Hits, Err5xx: p.Err5xx}
		if len(p.Lat) > 0 {
			row.P50 = analyzer.Percentile(p.Lat, 50)
			row.P95 = analyzer.Percentile(p.Lat, 95)
		}
		rep.Paths = append(rep.Paths, row)
	}

	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(os.Stderr, "logtop: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *format != "text" {
		fmt.Fprintf(os.Stderr, "logtop: unknown format %q (text|json)\n", *format)
		os.Exit(1)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "total\t%d\tskipped\t%d\n", rep.Total, rep.Skipped)
	fmt.Fprintln(w, "PATH\tHITS\t5XX\tP50ms\tP95ms")
	for _, p := range rep.Paths {
		fmt.Fprintf(w, "%s\t%d\t%d\t%.1f\t%.1f\n", p.Path, p.Hits, p.Err5xx, p.P50, p.P95)
	}
	w.Flush()
}
