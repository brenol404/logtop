// Package analyzer aggregates parsed log entries, sequentially or with a
// worker pool. Workers aggregate thread-local partial stats (no shared mutex
// on the hot path) and the partials merge at the end, so N workers produce
// byte-identical results to 1 worker.
package analyzer

import (
	"bufio"
	"io"
	"runtime"
	"sort"
	"sync"

	"github.com/brenol404/logtop/parser"
)

// PathStats aggregates one endpoint.
type PathStats struct {
	Hits   int
	Err5xx int
	Lat    []float64
}

// Stats is the full aggregation.
type Stats struct {
	Total   int
	Skipped int
	Status  map[int]int
	Paths   map[string]*PathStats
	LatAll  []float64
}

// Percentile returns the nearest-rank percentile (p in 0..100).
func Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	k := int((p / 100) * float64(len(cp)))
	if k < 1 {
		k = 1
	}
	if k > len(cp) {
		k = len(cp)
	}
	return cp[k-1]
}

// Aggregate folds entries sequentially (pure, deterministic).
func Aggregate(entries []parser.Entry) Stats {
	s := Stats{Status: map[int]int{}, Paths: map[string]*PathStats{}}
	for _, e := range entries {
		s.add(e)
	}
	return s
}

func (s *Stats) add(e parser.Entry) {
	s.Total++
	s.Status[e.Status]++
	ps := s.Paths[e.Path]
	if ps == nil {
		ps = &PathStats{}
		s.Paths[e.Path] = ps
	}
	ps.Hits++
	if e.Status >= 500 {
		ps.Err5xx++
	}
	if e.HasLatency {
		ps.Lat = append(ps.Lat, e.LatencyMs)
		s.LatAll = append(s.LatAll, e.LatencyMs)
	}
}

// merge folds another Stats into this one.
func (s *Stats) merge(o Stats) {
	s.Total += o.Total
	s.Skipped += o.Skipped
	for code, n := range o.Status {
		s.Status[code] += n
	}
	for path, ps := range o.Paths {
		mine := s.Paths[path]
		if mine == nil {
			mine = &PathStats{}
			s.Paths[path] = mine
		}
		mine.Hits += ps.Hits
		mine.Err5xx += ps.Err5xx
		mine.Lat = append(mine.Lat, ps.Lat...)
	}
	s.LatAll = append(s.LatAll, o.LatAll...)
}

// TopPaths returns the n busiest paths, ties broken by path for determinism.
func (s Stats) TopPaths(n int) []PathStat {
	out := make([]PathStat, 0, len(s.Paths))
	for path, ps := range s.Paths {
		out = append(out, PathStat{Path: path, Hits: ps.Hits, Err5xx: ps.Err5xx, Lat: ps.Lat})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].Path < out[j].Path
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// PathStat is a TopPaths row.
type PathStat struct {
	Path   string
	Hits   int
	Err5xx int
	Lat    []float64
}

// Run parses r and aggregates with workers goroutines (workers<=0 means NumCPU).
func Run(r io.Reader, workers int) (Stats, error) {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return Stats{}, err
	}
	if workers == 1 {
		return runChunk(lines), nil
	}
	const chunkSize = 10000
	var chunks [][]string
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, lines[i:end])
	}
	partials := make([]Stats, len(chunks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, chunk := range chunks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			partials[i] = runChunk(chunk)
		}()
	}
	wg.Wait()
	merged := Stats{Status: map[int]int{}, Paths: map[string]*PathStats{}}
	for _, p := range partials {
		merged.merge(p)
	}
	return merged, nil
}

func runChunk(lines []string) Stats {
	s := Stats{Status: map[int]int{}, Paths: map[string]*PathStats{}}
	for _, line := range lines {
		e, err := parser.ParseLine(line)
		if err != nil {
			s.Skipped++
			continue
		}
		s.add(e)
	}
	return s
}
