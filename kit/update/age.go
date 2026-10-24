// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	day  = 24 * time.Hour
	week = 7 * day
)

// ParseAge reads a release age such as "3d", "1w", "36h" or "1w2d": a Go
// duration (time.ParseDuration) that may also use d for days and w for
// weeks. "0" means no cooldown. Negative ages are refused.
func ParseAge(s string) (time.Duration, error) {
	in := strings.TrimSpace(s)
	bad := fmt.Errorf("invalid age %q; use a duration such as 3d, 1w or 36h", s)
	if in == "" {
		return 0, bad
	}
	if in == "0" {
		return 0, nil
	}
	var total time.Duration
	for rest := in; rest != ""; {
		i := strings.IndexFunc(rest, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
		if i <= 0 {
			return 0, bad // a missing number, a sign or a number with no unit
		}
		j := strings.IndexAny(rest[i:], "0123456789.")
		if j < 0 {
			j = len(rest) - i
		}
		num, unit := rest[:i], rest[i:i+j]
		rest = rest[i+j:]
		var d time.Duration
		switch unit {
		case "d", "w":
			f, err := strconv.ParseFloat(num, 64)
			if err != nil {
				return 0, bad
			}
			per := day
			if unit == "w" {
				per = week
			}
			if f*float64(per) >= math.MaxInt64 {
				return 0, bad
			}
			d = time.Duration(f * float64(per))
		default:
			var err error
			if d, err = time.ParseDuration(num + unit); err != nil {
				return 0, bad
			}
		}
		if total > math.MaxInt64-d {
			return 0, bad
		}
		total += d
	}
	return total, nil
}

// FormatAge writes d the way a person would say it: "1 day", "3 days",
// "36 hours", "1 week". Below two days a part day is shown in whole
// hours, so 36h doesn't read as "1 day". Ages under an hour are "less
// than an hour".
func FormatAge(d time.Duration) string {
	plural := func(n int64, unit string) string {
		if n == 1 {
			return "1 " + unit
		}
		return strconv.FormatInt(n, 10) + " " + unit + "s"
	}
	switch {
	case d < time.Hour:
		return "less than an hour"
	case d < day, d < 2*day && (d/time.Hour)%24 != 0:
		return plural(int64(d/time.Hour), "hour")
	case d%week == 0:
		return plural(int64(d/week), "week")
	}
	return plural(int64(d/day), "day")
}
