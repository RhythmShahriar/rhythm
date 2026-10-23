package cli

import (
	"bytes"
	"errors"
	"flag"
	"slices"
	"testing"
)

func TestParseInterspersed(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "")
	var tags ListFlag
	fs.Var(&tags, "tags", "")
	pos, err := ParseInterspersed(fs, []string{"a.example", "--json", "b.example", "--tags", "x, y", "--tags", "z"})
	if err != nil || !*asJSON || !slices.Equal(pos, []string{"a.example", "b.example"}) || !slices.Equal(tags, ListFlag{"x", "y", "z"}) {
		t.Errorf("pos=%v json=%v tags=%v err=%v", pos, *asJSON, tags, err)
	}
}

func TestUsageError(t *testing.T) {
	var ue UsageError
	if !errors.As(Usagef("bad %s", "flag"), &ue) || ue.Msg != "bad flag" {
		t.Errorf("got %v", ue)
	}
	if IsTerminal(&bytes.Buffer{}) {
		t.Error("a buffer isn't a terminal")
	}
}
