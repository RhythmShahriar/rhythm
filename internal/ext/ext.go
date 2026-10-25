// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

// Package ext finds and names third-party rhythm extensions.
//
// An extension is a GitHub repository owner/rhythm-<name> that carries
// the topic rhythm-extension and publishes releases the way Rhythm's own
// tools do (see kit/update's release contract): rhythm-<name>_<os>_<arch>
// archives, checksums.txt and a keyless Sigstore bundle over it,
// checksums.txt.sigstore.json. `rhythm <name>` then runs it.
//
// Who may sign an extension's releases is the extension's own business,
// within one rule: the signing workflow must belong to the extension's
// repository (a certificate SAN under https://github.com/<owner>/<repo>/,
// for that repository and the release tag), unless it is Rhythm's shared
// release workflow called from that repository. The first install
// records the workflow; every later release must come from the same one.
package ext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/update"
)

// Topic is the GitHub topic every extension repository carries.
const Topic = "rhythm-extension"

// RepoPrefix starts every extension repository's name.
const RepoPrefix = "rhythm-"

// SharedWorkflow is Rhythm's reusable release workflow, which an
// extension may call to sign its releases.
const SharedWorkflow = "https://github.com/RhythmShahriar/.github/.github/workflows/release-go.yml"

var (
	repoRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)
	// nameRE is the router's tool-name rule (cmd/rhythm validToolName).
	nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

// ValidName reports whether name may name an extension: the same rule
// as every tool name, so `rhythm <name>` finds rhythm-<name> on PATH and
// nothing else.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// Spec is what rhythm ext install takes: owner/rhythm-name[@version].
type Spec struct {
	Repo    string // owner/rhythm-name, as typed
	Name    string // name, what `rhythm <name>` runs
	Version string // the version asked for; "" for the latest
}

// ParseSpec reads owner/rhythm-name[@version].
func ParseSpec(s string) (Spec, error) {
	repo, v, pinned := strings.Cut(strings.TrimSpace(s), "@")
	if pinned && v == "" {
		return Spec{}, fmt.Errorf("%q names no version after the @", s)
	}
	if !repoRE.MatchString(repo) {
		return Spec{}, fmt.Errorf("%q isn't owner/rhythm-<name>, such as someone/rhythm-zap", s)
	}
	_, base, _ := strings.Cut(repo, "/")
	name, ok := strings.CutPrefix(base, RepoPrefix)
	switch {
	case !ok || name == "":
		return Spec{}, fmt.Errorf("%s isn't an extension repository: its name must start with %s, such as someone/rhythm-zap", repo, RepoPrefix)
	case !ValidName(name):
		return Spec{}, fmt.Errorf("%s: %q isn't a valid tool name (lowercase letters, digits and dashes)", repo, name)
	}
	return Spec{Repo: repo, Name: name, Version: v}, nil
}

// SignerAllowed reports whether signer, a certificate SAN, may sign
// releases of repo: a workflow of repo itself or the shared workflow.
// The shared workflow's SAN is the same for every repository that calls
// it, so this check alone doesn't tie the signature to repo; the
// certificate's source repository and tag, which kit/sigverify checks,
// do that.
func SignerAllowed(repo, signer string) bool {
	return strings.HasPrefix(signer, "https://github.com/"+repo+"/") || strings.HasPrefix(signer, SharedWorkflow+"@")
}

// Repo is one search result.
type Repo struct {
	FullName    string `json:"full_name"`   // owner/rhythm-name
	Name        string `json:"name"`        // the extension name: the repo name without rhythm-
	Description string `json:"description"` // the repository's own, untrusted text
	Stars       int    `json:"stars"`       // stargazers, the search's sort order
	URL         string `json:"url"`         // the repository's page on GitHub
}

// Client asks the GitHub search API.
type Client struct {
	API       string       // API base; "" means https://api.github.com
	HTTP      *http.Client // nil means http.DefaultClient
	Token     string       // sent as a bearer token, chosen by the caller's token rules
	UserAgent string       // GitHub refuses API requests without one
}

// ErrSearch is a search the API refused, such as a query it can't parse.
type ErrSearch struct {
	Status  int    // the HTTP status code
	Message string // the API's "message" field, "" when the body had none
}

func (e *ErrSearch) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("the search API refused the search (HTTP %d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("the search API refused the search (HTTP %d)", e.Status)
}

// Search finds extension repositories matching words, most starred
// first. Repositories whose names don't follow the rhythm-<name> rule
// are left out: rhythm couldn't run them.
func (c *Client) Search(ctx context.Context, words []string) ([]Repo, error) {
	api := strings.TrimSuffix(c.API, "/")
	if api == "" {
		api = "https://api.github.com"
	}
	// The same rules as the release API (https, no user info or query),
	// since Token goes to whatever host this names.
	if err := update.CheckAPI(api); err != nil {
		return nil, fmt.Errorf("search API: %w", err)
	}
	q := strings.TrimSpace(strings.Join(words, " ") + " topic:" + Topic)
	u := api + "/search/repositories?" + url.Values{"q": {q}, "sort": {"stars"}, "order": {"desc"}, "per_page": {"30"}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		return nil, &ErrSearch{Status: resp.StatusCode, Message: e.Message}
	}
	var found struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			HTMLURL     string `json:"html_url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &found); err != nil {
		return nil, fmt.Errorf("reading the search results: %w", err)
	}
	out := []Repo{}
	for _, it := range found.Items {
		s, err := ParseSpec(it.FullName)
		if err != nil {
			continue
		}
		out = append(out, Repo{FullName: it.FullName, Name: s.Name, Description: it.Description, Stars: it.Stars, URL: it.HTMLURL})
	}
	return out, nil
}
