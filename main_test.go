package main

import (
	"testing"
	"time"
)

func TestParseFriendlyDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"0", 0, false},
		{"", 0, false},
		{"365d", 365 * 24 * time.Hour, false},
		{"180d", 180 * 24 * time.Hour, false},
		{"72h", 72 * time.Hour, false},
		{"1.5d", 36 * time.Hour, false},
		{"garbage", 0, true},
		{"-5d", 0, true},
	}
	for _, tc := range cases {
		got, err := parseFriendlyDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseFriendlyDuration(%q): expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseFriendlyDuration(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseFriendlyDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseUint(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 0},
		{"0", 0},
		{"12345", 12345},
		{"not-a-number", 0},
		{"-5", 0},
	}
	for _, tc := range cases {
		if got := parseUint(tc.in); got != tc.want {
			t.Errorf("parseUint(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestComputeMetrics(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	published := now.Add(-10 * 24 * time.Hour) // 10 days old

	ageDays, ratio, viewsPerDay := computeMetrics(40000, 2000, false, published, now)
	if absFloat(ageDays-10) > 0.001 {
		t.Errorf("ageDays = %v, want ~10", ageDays)
	}
	if ratio != 20 {
		t.Errorf("ratio = %v, want 20", ratio)
	}
	if viewsPerDay != 4000 {
		t.Errorf("viewsPerDay = %v, want 4000", viewsPerDay)
	}

	// Hidden subscriber count: ratio must be unavailable (0), not divide-by-zero.
	_, hiddenRatio, _ := computeMetrics(40000, 2000, true, published, now)
	if hiddenRatio != 0 {
		t.Errorf("hiddenRatio = %v, want 0", hiddenRatio)
	}

	// Zero subscribers: ratio must be 0, not a division panic or Inf.
	_, zeroSubRatio, _ := computeMetrics(40000, 0, false, published, now)
	if zeroSubRatio != 0 {
		t.Errorf("zeroSubRatio = %v, want 0", zeroSubRatio)
	}

	// A video published this instant should floor at a small age rather
	// than dividing by zero.
	freshAgeDays, _, freshViewsPerDay := computeMetrics(100, 100, false, now, now)
	if freshAgeDays <= 0 {
		t.Errorf("freshAgeDays = %v, want > 0", freshAgeDays)
	}
	if freshViewsPerDay <= 100 {
		t.Errorf("freshViewsPerDay = %v, want > 100 (views/day should spike for brand-new videos)", freshViewsPerDay)
	}
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func TestIsOutlier(t *testing.T) {
	cfg := config{MaxSubs: 20000, MinViews: 10000, MinRatio: 3}

	cases := []struct {
		name string
		v    videoStats
		want bool
	}{
		{
			name: "clear outlier",
			v:    videoStats{Subscribers: 2000, Views: 40000, ViewsPerSub: 20},
			want: true,
		},
		{
			name: "hidden subscriber count excluded",
			v:    videoStats{SubscribersHidden: true, Subscribers: 2000, Views: 40000, ViewsPerSub: 20},
			want: false,
		},
		{
			name: "zero subscribers excluded",
			v:    videoStats{Subscribers: 0, Views: 40000, ViewsPerSub: 0},
			want: false,
		},
		{
			name: "channel too big",
			v:    videoStats{Subscribers: 50000, Views: 40000, ViewsPerSub: 3},
			want: false,
		},
		{
			name: "not enough views",
			v:    videoStats{Subscribers: 2000, Views: 100, ViewsPerSub: 3},
			want: false,
		},
		{
			name: "ratio too low",
			v:    videoStats{Subscribers: 5000, Views: 10000, ViewsPerSub: 2},
			want: false,
		},
		{
			name: "exactly at thresholds",
			v:    videoStats{Subscribers: 20000, Views: 10000, ViewsPerSub: 3},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOutlier(tc.v, cfg); got != tc.want {
				t.Errorf("isOutlier(%+v) = %v, want %v", tc.v, got, tc.want)
			}
		})
	}
}

func TestAppendUnique(t *testing.T) {
	s := []string{"a", "b"}
	s = appendUnique(s, "b")
	if len(s) != 2 {
		t.Errorf("appendUnique should not duplicate an existing entry, got %v", s)
	}
	s = appendUnique(s, "c")
	if len(s) != 3 || s[2] != "c" {
		t.Errorf("appendUnique should add a new entry, got %v", s)
	}
}

func TestValidSearchOrder(t *testing.T) {
	for _, ok := range []string{"relevance", "date", "rating", "title", "videoCount", "viewCount"} {
		if !validSearchOrder(ok) {
			t.Errorf("validSearchOrder(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "popularity", "RELEVANCE"} {
		if validSearchOrder(bad) {
			t.Errorf("validSearchOrder(%q) = true, want false", bad)
		}
	}
}
