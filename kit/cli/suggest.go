// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package cli

// Suggest returns the option closest to input, for a "did you mean"
// hint, or "" when nothing is close. Close means at most two edits
// (insertions, deletions or substitutions) and fewer edits than input
// has characters, so "x" suggests nothing rather than every one-letter
// alias. Ties go to the earlier option; an exact match is returned as is.
func Suggest(input string, options []string) string {
	in := []rune(input)
	best, bestDist := "", 3
	for _, o := range options {
		d := levenshtein(in, []rune(o))
		if d < bestDist && d < len(in) || d == 0 {
			best, bestDist = o, d
			if d == 0 {
				break
			}
		}
	}
	return best
}

// levenshtein returns the edit distance between a and b.
func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
