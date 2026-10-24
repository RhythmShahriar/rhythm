// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseAge(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"0", 0},
		{"3d", 3 * day},
		{"1w", week},
		{"36h", 36 * time.Hour},
		{"1w2d", 9 * day},
		{"1d12h", 36 * time.Hour},
		{"1.5d", 36 * time.Hour},
		{"90m", 90 * time.Minute},
		{" 2d ", 2 * day},
		{"0d", 0},
	} {
		got, err := ParseAge(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseAge(%q) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "3", "d", "-3d", "3 d", "3x", "3days", "1..5d", "99999999999w", "9223372036s1d"} {
		if got, err := ParseAge(in); err == nil || !strings.Contains(err.Error(), "invalid age") {
			t.Errorf("ParseAge(%q) = %v, %v; want an error", in, got, err)
		}
	}
}

func TestFormatAge(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Minute:  "less than an hour",
		time.Hour:         "1 hour",
		5 * time.Hour:     "5 hours",
		day:               "1 day",
		36 * time.Hour:    "36 hours",
		3 * day:           "3 days",
		3*day + time.Hour: "3 days",
		week:              "1 week",
		2 * week:          "2 weeks",
		10 * day:          "10 days",
	} {
		if got := FormatAge(d); got != want {
			t.Errorf("FormatAge(%v) = %q, want %q", d, got, want)
		}
	}
}

// cooled serves 0.3.2 published 10 days ago and 0.3.3 one day ago.
func cooled(t *testing.T) *Updater {
	api := newReleaseAPI(t,
		fakeRel{tag: "v0.3.2", published: time.Now().Add(-10 * day)},
		fakeRel{tag: "v0.3.3", published: time.Now().Add(-day)},
		fakeRel{tag: "v0.4.0-rc1", pre: true, published: time.Now().Add(-20 * day)},
	)
	return api.updater()
}

func TestMinAgeSkipsYoungReleases(t *testing.T) {
	u := cooled(t)
	u.MinAge = 3 * day
	for _, spec := range []string{"latest", "0.3", "0"} {
		rel, err := u.Resolve(context.Background(), spec)
		if err != nil || rel.Version != "0.3.2" {
			t.Fatalf("Resolve(%q) = %+v, %v; want 0.3.2", spec, rel, err)
		}
		s := rel.SkippedNewer
		if s == nil || s.Version != "0.3.3" || s.Age < day || s.Age > day+time.Minute {
			t.Errorf("Resolve(%q) SkippedNewer = %+v", spec, s)
		}
	}
	rel, err := u.Latest(context.Background())
	if err != nil || rel.Version != "0.3.2" || rel.SkippedNewer == nil {
		t.Errorf("Latest = %+v, %v", rel, err)
	}
	// An exact version is the user's choice, whatever its age.
	rel, err = u.Resolve(context.Background(), "0.3.3")
	if err != nil || rel.Version != "0.3.3" || rel.SkippedNewer != nil {
		t.Errorf("Resolve(0.3.3) = %+v, %v", rel, err)
	}
}

func TestMinAgeZeroTakesTheNewest(t *testing.T) {
	u := cooled(t)
	for _, spec := range []string{"latest", "0.3"} {
		rel, err := u.Resolve(context.Background(), spec)
		if err != nil || rel.Version != "0.3.3" || rel.SkippedNewer != nil {
			t.Errorf("Resolve(%q) = %+v, %v; want 0.3.3", spec, rel, err)
		}
	}
	// Old enough already: nothing is skipped.
	u.MinAge = 12 * time.Hour
	rel, err := u.Latest(context.Background())
	if err != nil || rel.Version != "0.3.3" || rel.SkippedNewer != nil {
		t.Errorf("Latest with 12h = %+v, %v", rel, err)
	}
}

func TestMinAgeWithNothingOldEnough(t *testing.T) {
	u := cooled(t)
	u.MinAge = 2 * week
	_, err := u.Latest(context.Background())
	var tn *ErrTooNew
	if !errors.As(err, &tn) || tn.Newest != "0.3.3" {
		t.Fatalf("err = %v, want ErrTooNew", err)
	}
	if want := "no release of RhythmShahriar/tuner is old enough: the newest, 0.3.3, is 1 day old (min age 2 weeks)"; err.Error() != want {
		t.Errorf("message = %q", err)
	}
}
