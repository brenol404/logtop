package parser

import (
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    Entry
		wantErr bool
	}{
		{
			name: "common log format",
			line: `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326`,
			want: Entry{
				Host: "127.0.0.1", Method: "GET", Path: "/apache_pb.gif",
				Status: 200, Size: 2326, HasLatency: false,
				Time: time.Date(2000, 10, 10, 13, 55, 36, 0, time.FixedZone("", -7*3600)),
			},
		},
		{
			name: "combined with latency",
			line: `10.0.0.2 - - [11/Oct/2000:09:00:00 +0000] "POST /api/v1/users HTTP/1.1" 201 512 "http://ref" "curl/8.0" 12345`,
			want: Entry{
				Host: "10.0.0.2", Method: "POST", Path: "/api/v1/users",
				Status: 201, Size: 512, LatencyMs: 12.345, HasLatency: true,
				Time: time.Date(2000, 10, 11, 9, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "bare latency without referrer",
			line: `10.0.0.3 - - [11/Oct/2000:09:00:01 +0000] "GET /slow HTTP/1.1" 500 0 250000`,
			want: Entry{
				Host: "10.0.0.3", Method: "GET", Path: "/slow",
				Status: 500, Size: 0, LatencyMs: 250.0, HasLatency: true,
				Time: time.Date(2000, 10, 11, 9, 0, 1, 0, time.UTC),
			},
		},
		{
			name: "dash size means zero",
			line: `10.0.0.4 - - [11/Oct/2000:09:00:02 +0000] "GET /empty HTTP/1.1" 304 -`,
			want: Entry{
				Host: "10.0.0.4", Method: "GET", Path: "/empty", Status: 304,
				Time: time.Date(2000, 10, 11, 9, 0, 2, 0, time.UTC),
			},
		},
		{
			name:    "garbage rejected",
			line:    "this is not a log line",
			wantErr: true,
		},
		{
			name:    "bad status rejected",
			line:    `10.0.0.5 - - [11/Oct/2000:09:00:03 +0000] "GET /x HTTP/1.1" OK 10`,
			wantErr: true,
		},
		{
			name:    "trailing garbage rejected",
			line:    `10.0.0.5 - - [11/Oct/2000:09:00:03 +0000] "GET /x HTTP/1.1" 200 10 foo bar`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLine(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Host != tt.want.Host || got.Method != tt.want.Method ||
				got.Path != tt.want.Path || got.Status != tt.want.Status ||
				got.Size != tt.want.Size || got.HasLatency != tt.want.HasLatency {
				t.Errorf("mismatch:\n got %+v\nwant %+v", got, tt.want)
			}
			if got.HasLatency && got.LatencyMs != tt.want.LatencyMs {
				t.Errorf("latency: got %v want %v", got.LatencyMs, tt.want.LatencyMs)
			}
			if !got.Time.Equal(tt.want.Time) {
				t.Errorf("time: got %v want %v", got.Time, tt.want.Time)
			}
		})
	}
}
