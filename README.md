# logtop

[![CI](https://github.com/brenol404/logtop/actions/workflows/ci.yml/badge.svg)](https://github.com/brenol404/logtop/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.23%2B-blue)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> Concurrent Apache/Nginx access-log analyzer: top paths, status histogram and latency percentiles. Single binary, stdin/stdout.

> **Evidence:** 1M lines in **0.72s** (8 workers) vs 1.94s sequential · table-driven tests incl. 1-vs-N determinism · `go vet` + `gofmt` in CI.

## Usage

```bash
go build -o logtop .

logtop access.log --top 10 --workers 8
cat access.log | logtop --format json --top 5
```

Flags: `--top N` (default 10), `--workers N` (default NumCPU, `1` = sequential),
`--format text|json` (default text). Reads a file argument or stdin.

Input: Common or Combined Log Format, with an optional trailing `%D`
latency field (microseconds). Malformed lines are counted as `skipped`,
never fatal:

```
total           1000000  skipped  0
PATH            HITS     5XX      P50ms  P95ms
/login          167285   0        398.5  760.3
/api/orders     167027   0        401.8  760.2
```

## Benchmark (1M synthetic lines, 4GB box)

| mode | wall | user CPU |
|---|---|---|
| 1 worker | 1.94s | 1.88s |
| 8 workers | **0.72s** (2.7x) | 2.78s |

Honest bottleneck: the read phase is sequential (single scanner), so workers
parallelize only parse+aggregate. A streaming pipeline would push this further —
left as future work, not hidden.

## Design

Workers aggregate thread-local partial stats (no shared mutex on the hot path)
and merge at the end — N workers produce byte-identical results to 1 worker,
pinned by test. Top-N ties break by path for determinism.

## Structure

```
.
├── main.go              # CLI (flags, file/stdin, text|json output)
├── parser/              # CLF/combined/%D line → struct (+ table tests)
├── analyzer/            # sequential + worker-pool aggregation (+ tests)
└── .github/workflows/   # gofmt + vet + test + build
```

## Quality

- Table-driven parser tests (CLF, combined, latency variants, malformed).
- Determinism test: 1 vs 4 workers, identical output.
- CI: `gofmt`, `go vet`, `go test`, `go build`. Zero dependencies (stdlib only).

## License

MIT.
