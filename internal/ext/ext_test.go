// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package ext

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSpec(t *testing.T) {
	for in, want := range map[string]Spec{
		"someone/rhythm-zap":        {Repo: "someone/rhythm-zap", Name: "zap"},
		"someone/rhythm-zap@1.2.0":  {Repo: "someone/rhythm-zap", Name: "zap", Version: "1.2.0"},
		"Some-One/rhythm-zap2@v1.2": {Repo: "Some-One/rhythm-zap2", Name: "zap2", Version: "v1.2"},
	} {
		got, err := ParseSpec(in)
		if err != nil || got != want {
			t.Errorf("ParseSpec(%q) = %+v, %v", in, got, err)
		}
	}
	for in, want := range map[string]string{
		"zap":                    "isn't owner/rhythm-<name>",
		"someone/zap":            "its name must start with rhythm-",
		"someone/rhythm-":        "its name must start with rhythm-",
		"someone/rhythm-Zap":     `"Zap" isn't a valid tool name`,
		"someone/rhythm-z.p":     `"z.p" isn't a valid tool name`,
		"someone/rhythm-zap@":    "names no version",
		"../rhythm-zap":          "isn't owner/rhythm-<name>",
		"a/b/rhythm-zap":         "isn't owner/rhythm-<name>",
		"https://github.com/x/y": "isn't owner/rhythm-<name>",
	} {
		if _, err := ParseSpec(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseSpec(%q) = %v, want %q", in, err, want)
		}
	}
}

func TestSignerAllowed(t *testing.T) {
	const repo = "someone/rhythm-zap"
	for signer, ok := range map[string]bool{
		"https://github.com/someone/rhythm-zap/.github/workflows/release.yml@refs/tags/v1.0.0":    true,
		SharedWorkflow + "@refs/heads/master":                                                     true,
		"https://github.com/someone/rhythm-zapper/.github/workflows/release.yml@refs/tags/v1.0.0": false,
		"https://github.com/mallory/rhythm-zap/.github/workflows/release.yml@refs/tags/v1.0.0":    false,
		"someone@example.com": false,
	} {
		if got := SignerAllowed(repo, signer); got != ok {
			t.Errorf("SignerAllowed(%q) = %v", signer, got)
		}
	}
}

func TestSearch(t *testing.T) {
	var gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			http.NotFound(w, r)
			return
		}
		gotQuery, gotAuth = r.URL.Query().Get("q"), r.Header.Get("Authorization")
		if strings.Contains(gotQuery, "bad") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
			return
		}
		_, _ = w.Write([]byte(`{"total_count":3,"items":[
			{"full_name":"someone/rhythm-zap","name":"rhythm-zap","description":"zaps things","stargazers_count":42,"html_url":"https://github.com/someone/rhythm-zap"},
			{"full_name":"other/not-an-extension","name":"not-an-extension","description":"wrong name","stargazers_count":99},
			{"full_name":"other/rhythm-pdf","name":"rhythm-pdf","description":"","stargazers_count":3}]}`))
	}))
	t.Cleanup(srv.Close)
	c := &Client{API: srv.URL + "/", Token: "tok", UserAgent: "test"}
	got, err := c.Search(context.Background(), []string{"zap", "things"})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "zap things topic:rhythm-extension" || gotAuth != "Bearer tok" {
		t.Errorf("query %q auth %q", gotQuery, gotAuth)
	}
	if len(got) != 2 || got[0].Name != "zap" || got[0].Stars != 42 || got[1].FullName != "other/rhythm-pdf" {
		t.Errorf("results %+v", got)
	}
	// No words searches the topic alone.
	if _, err := c.Search(context.Background(), nil); err != nil || gotQuery != "topic:rhythm-extension" {
		t.Errorf("no words: %q, %v", gotQuery, err)
	}
	var se *ErrSearch
	if _, err := c.Search(context.Background(), []string{"bad"}); !errors.As(err, &se) || se.Status != 422 || !strings.Contains(err.Error(), "Validation Failed") {
		t.Errorf("refused search: %v", err)
	}
	// The API base follows the same rule as RHYTHM_GITHUB_API.
	if _, err := (&Client{API: "http://example.com"}).Search(context.Background(), nil); err == nil {
		t.Error("searched over plain http to another host")
	}
}
