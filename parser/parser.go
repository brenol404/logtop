// Package parser reads Apache/Nginx access logs in Common or Combined Log
// Format, with an optional trailing latency field (%D microseconds):
//
//	127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /x HTTP/1.0" 200 2326
//	127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "POST /api HTTP/1.1" 201 512 "http://ref" "curl/8" 12345
//
// Malformed lines return an error (the caller counts them as skipped).
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Entry is one parsed log line.
type Entry struct {
	Host       string
	Time       time.Time
	Method     string
	Path       string
	Status     int
	Size       int64
	LatencyMs  float64
	HasLatency bool
}

var clfRe = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+)(?: ([^ ]+) [^"]*)?" (\d{3}) (\S+)(.*)$`)

const timeFormat = "02/Jan/2006:15:04:05 -0700"

// ParseLine parses one log line in CLF, Combined, or either suffixed with %D.
func ParseLine(line string) (Entry, error) {
	var e Entry
	m := clfRe.FindStringSubmatch(line)
	if m == nil {
		return e, fmt.Errorf("not a log line: %q", truncate(line))
	}
	t, err := time.Parse(timeFormat, m[2])
	if err != nil {
		return e, fmt.Errorf("bad timestamp %q: %w", m[2], err)
	}
	status, err := strconv.Atoi(m[5])
	if err != nil {
		return e, fmt.Errorf("bad status %q: %w", m[5], err)
	}
	var size int64
	if m[6] != "-" {
		size, err = strconv.ParseInt(m[6], 10, 64)
		if err != nil {
			return e, fmt.Errorf("bad size %q: %w", m[6], err)
		}
	}
	e = Entry{
		Host:   m[1],
		Time:   t,
		Method: m[3],
		Path:   m[4],
		Status: status,
		Size:   size,
	}
	rest := strings.TrimSpace(m[7])
	for i := 0; i < 2 && strings.HasPrefix(rest, "\""); i++ {
		end := strings.Index(rest[1:], "\"")
		if end < 0 {
			return e, fmt.Errorf("unterminated quoted string: %q", truncate(line))
		}
		rest = strings.TrimSpace(rest[end+2:])
	}
	if fields := strings.Fields(rest); len(fields) == 1 {
		us, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return e, fmt.Errorf("bad latency %q: %w", fields[0], err)
		}
		e.LatencyMs, e.HasLatency = float64(us)/1000.0, true
	} else if len(fields) > 1 {
		return e, fmt.Errorf("trailing garbage: %q", truncate(line))
	}
	return e, nil
}

func truncate(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}
