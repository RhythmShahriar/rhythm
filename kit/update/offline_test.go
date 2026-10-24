// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package update

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// isolate points every config dir the OS may use at a temp dir and clears
// the switches that turn update checks off, so tests see a clean machine.
// XDG_CONFIG_HOME alone is not enough: macOS ignores it.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("APPDATA", filepath.Join(dir, "appdata"))
	t.Setenv("RHYTHM_CONFIG_DIR", filepath.Join(dir, "rhythm"))
	t.Setenv("CI", "")
	t.Setenv("RHYTHM_NO_UPDATE_CHECK", "")
	t.Setenv("RHYTHM_OFFLINE", "")
}

// counting serves a release and counts the requests it gets.
func counting(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"tag_name":"v0.3.0","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestOfflineNeverTouchesTheNetwork(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     string
		offline bool
	}{
		{"RHYTHM_OFFLINE=1", "1", false},
		{"RHYTHM_OFFLINE=yes", "yes", false},
		{"Offline field", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			t.Setenv("RHYTHM_OFFLINE", tc.env)
			srv, hits := counting(t)
			u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL, Offline: tc.offline}

			if latest, newer := u.Check(context.Background()); latest != "" || newer {
				t.Errorf("Check = %q, %v", latest, newer)
			}
			ch := Notice(context.Background(), u)
			select {
			case msg, ok := <-ch:
				if ok {
					t.Errorf("Notice sent %q", msg)
				}
			default:
				t.Error("Notice did not close its channel at once")
			}
			if _, err := u.Latest(context.Background()); !errors.Is(err, ErrOffline) {
				t.Errorf("Latest err = %v, want ErrOffline", err)
			}
			rel := &Release{Version: "0.3.0", Assets: map[string]string{u.ArchiveName(): srv.URL + "/a", "checksums.txt": srv.URL + "/s"}}
			if err := u.Install(context.Background(), rel, filepath.Join(t.TempDir(), "bin")); !errors.Is(err, ErrOffline) {
				t.Errorf("Install err = %v, want ErrOffline", err)
			}
			if n := hits.Load(); n != 0 {
				t.Errorf("%d requests while offline", n)
			}
		})
	}
}

func TestOfflineZeroKeepsChecksOn(t *testing.T) {
	isolate(t)
	t.Setenv("RHYTHM_OFFLINE", "0")
	if CheckDisabled("0.2.0") {
		t.Error("RHYTHM_OFFLINE=0 disabled update checks")
	}
	srv, hits := counting(t)
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", Current: "0.2.0", API: srv.URL}
	if latest, newer := u.Check(context.Background()); latest != "0.3.0" || !newer {
		t.Errorf("Check = %q, %v", latest, newer)
	}
	if hits.Load() != 1 {
		t.Errorf("%d requests, want 1", hits.Load())
	}
}

func TestErrOfflineMessage(t *testing.T) {
	if got := ErrOffline.Error(); got != "offline mode is on; unset RHYTHM_OFFLINE to download" {
		t.Errorf("ErrOffline = %q", got)
	}
}

func TestUserAgentIsTheUpdaters(t *testing.T) {
	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("User-Agent"))
		w.Write([]byte(`{"tag_name":"v0.3.0","assets":[]}`))
	}))
	defer srv.Close()
	u := &Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner", API: srv.URL, UserAgent: "rhythm/1.2.3"}
	if _, err := u.Latest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.Load() != "rhythm/1.2.3" {
		t.Errorf("User-Agent = %v", got.Load())
	}
	u.UserAgent = ""
	if _, err := u.Latest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got.Load() != "rhythm-update/rhythm-tuner" {
		t.Errorf("default User-Agent = %v", got.Load())
	}
}

// TestDefaultClientUsesHTTPSProxy runs Latest in a child process, because
// Go reads the proxy variables once per process. The proxy on 127.0.0.1
// refuses the CONNECT, so nothing leaves the machine. The child's
// transport dials only loopback addresses, so a proxy that is ignored
// fails the test instead of reaching GitHub.
func TestDefaultClientUsesHTTPSProxy(t *testing.T) {
	if os.Getenv("UPDATE_PROXY_CHILD") == "1" {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			t.Fatal("http.DefaultTransport isn't an *http.Transport")
		}
		dialer := &net.Dialer{}
		base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
				return nil, fmt.Errorf("test dials only loopback, not %s", addr)
			}
			return dialer.DialContext(ctx, network, addr)
		}
		_, err := (&Updater{Repo: "RhythmShahriar/tuner", Binary: "rhythm-tuner"}).Latest(context.Background())
		fmt.Println("child:", err)
		return
	}
	var connect atomic.Value
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connect.Store(r.Method + " " + r.Host)
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer proxy.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDefaultClientUsesHTTPSProxy$")
	cmd.Env = append(os.Environ(), "UPDATE_PROXY_CHILD=1", "HTTPS_PROXY="+proxy.URL, "https_proxy="+proxy.URL,
		"NO_PROXY=", "no_proxy=", "RHYTHM_OFFLINE=", "GITHUB_TOKEN=", "RHYTHM_GITHUB_TOKEN=", "RHYTHM_GITHUB_API=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if got := connect.Load(); got != "CONNECT api.github.com:443" {
		t.Errorf("proxy saw %v, want a CONNECT to api.github.com\n%s", got, out)
	}
}
