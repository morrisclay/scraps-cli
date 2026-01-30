package cli

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			s:      "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncate with ellipsis",
			s:      "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "very short maxLen",
			s:      "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "empty string",
			s:      "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name    string
		dateStr string
		want    string
	}{
		{
			name:    "valid RFC3339",
			dateStr: "2024-03-15T10:30:00Z",
			want:    "Mar 15, 2024",
		},
		{
			name:    "invalid format",
			dateStr: "not-a-date",
			want:    "not-a-date",
		},
		{
			name:    "empty string",
			dateStr: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDate(tt.dateStr)
			if got != tt.want {
				t.Errorf("formatDate(%q) = %q, want %q", tt.dateStr, got, tt.want)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		name    string
		dateStr string
		want    string
	}{
		{
			name:    "valid RFC3339",
			dateStr: "2024-03-15T10:30:00Z",
			want:    "Mar 15, 2024 10:30",
		},
		{
			name:    "invalid format",
			dateStr: "not-a-date",
			want:    "not-a-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDateTime(tt.dateStr)
			if got != tt.want {
				t.Errorf("formatDateTime(%q) = %q, want %q", tt.dateStr, got, tt.want)
			}
		})
	}
}
