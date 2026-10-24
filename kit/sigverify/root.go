// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package sigverify

import (
	"context"
	_ "embed"
	"net/http"
	"path/filepath"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"

	"github.com/RhythmShahriar/rhythm/kit/config"
)

// embeddedRoot is the public-good Sigstore trusted root (Fulcio CAs, Rekor
// and CT log keys, timestamp authorities) as published through TUF.
//
//go:embed trusted_root.json
var embeddedRoot []byte

// tufTimeout bounds each TUF metadata request.
const tufTimeout = 10 * time.Second

// loadRoot returns the trusted root and where it came from. Online it asks
// the TUF repository, which reuses metadata cached in the last day; any
// failure there (offline, blocked, mirror down) falls back to the embedded
// root, because a release signed today chains to keys that root already
// holds and refusing every install would be worse. Options.OnFallback
// hears about that fallback so it is never invisible to the caller.
func loadRoot(ctx context.Context, opts Options) (root.TrustedMaterial, string, error) {
	if !opts.Offline {
		tr, err := fetchRoot(ctx, opts)
		if err == nil {
			return tr, RootTUF, nil
		}
		if opts.OnFallback != nil {
			opts.OnFallback(err)
		}
	}
	tr, err := root.NewTrustedRootFromJSON(embeddedRoot)
	return tr, RootEmbedded, err
}

func fetchRoot(ctx context.Context, opts Options) (*root.TrustedRoot, error) {
	dir := opts.CacheDir
	if dir == "" {
		d, err := config.CacheDir("rhythm")
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(d, "tuf")
	}
	t := tuf.DefaultOptions()
	t.CachePath = dir
	t.CacheValidity = 1 // days
	if opts.Mirror != "" {
		t.RepositoryBaseURL = opts.Mirror
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: tufTimeout}
	}
	f := fetcher.NewDefaultFetcher()
	f.SetHTTPClient(ctxClient{ctx: ctx, hc: hc})
	t.Fetcher = f
	t.Context = ctx
	c, err := tuf.New(t)
	if err != nil {
		return nil, err
	}
	return root.GetTrustedRoot(c)
}

// ctxClient attaches ctx to each TUF request, which the TUF fetcher API
// has no other way to take.
type ctxClient struct {
	ctx context.Context
	hc  *http.Client
}

func (c ctxClient) Do(req *http.Request) (*http.Response, error) {
	return c.hc.Do(req.WithContext(c.ctx)) //nolint:gosec // G704: the URL is the configured TUF mirror
}
