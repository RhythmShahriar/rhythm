// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var regenerate = flag.Bool("golden", false, "rewrite the testdata/*.golden.json files")

// golden compares got, with the sandbox paths and the fake API's address
// replaced by placeholders, with testdata/name.
func golden(t *testing.T, name, got string, replace ...string) {
	t.Helper()
	got = strings.NewReplacer(replace...).Replace(got)
	p := filepath.Join("testdata", name)
	if *regenerate {
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs; got\n%s\nwant\n%s", name, got, want)
	}
}

// decodeStrict decodes data into v, refusing unknown fields, and returns
// the keys of every object in arrays under field, to check that each
// required key is present.
func decodeStrict(t *testing.T, data string, v any, field string) []map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	var raw map[string][]map[string]any
	var loose map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &loose); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"x":`+string(loose[field])+`}`), &raw); err != nil {
		t.Fatalf("%s isn't an array of objects: %v", field, err)
	}
	return raw["x"]
}

func requireKeys(t *testing.T, objs []map[string]any, keys ...string) {
	t.Helper()
	for i, o := range objs {
		for _, k := range keys {
			if _, ok := o[k]; !ok {
				t.Errorf("item %d has no %q: %v", i, k, o)
			}
		}
	}
}

func TestUpdateCheckJSON(t *testing.T) {
	installed, latest := allTools()
	latest[tools["echo"]] = "0.1.0"
	delete(latest, tools["soundcheck"]) // its lookup fails
	latest[selfRepo] = "0.1.0"
	s := newSandbox(t, installed, latest)
	fakeSelf(t, s.stub(t, "rhythm", "0.1.0", ""), "0.1.0")

	code, out, errs := runCmd("update", "--check", "--json")
	if code != 1 {
		t.Errorf("exit %d\n%s", code, errs)
	}
	golden(t, "update-check-failed.golden.json", out, s.dir, "$SANDBOX", s.api.URL, "$API")
	if !strings.Contains(errs, "rhythm-tuner       0.1.0 → 0.2.0 available") || strings.Contains(out, "available\n") {
		t.Errorf("the human lines aren't on stderr alone:\n%s", errs)
	}

	var rep updateReportJSON
	objs := decodeStrict(t, out, &rep, "targets")
	if rep.Schema != "rhythm-update/v1" || len(rep.Targets) != 4 {
		t.Errorf("%+v", rep)
	}
	requireKeys(t, objs, "name", "current", "latest", "path", "outdated", "signed", "managed_by", "error")
}

func TestUpdateCheckJSONOutdatedExits10(t *testing.T) {
	installed, latest := allTools()
	latest[tools["echo"]], latest[tools["soundcheck"]] = "0.1.0", "0.1.0"
	s := newSandbox(t, installed, latest)

	code, out, errs := runCmd("update", "--check", "--json", "tuner", "echo")
	if code != exitOutdated {
		t.Errorf("exit %d\n%s", code, errs)
	}
	golden(t, "update-check-outdated.golden.json", out, s.dir, "$SANDBOX")
	var rep updateReportJSON
	requireKeys(t, decodeStrict(t, out, &rep, "targets"), "name", "current", "latest", "path", "outdated", "signed", "managed_by", "error")
	if len(rep.Targets) != 2 || !rep.Targets[1].Outdated || rep.Targets[0].Outdated {
		t.Errorf("%+v", rep.Targets)
	}
}

func TestUpdateJSONNeedsCheck(t *testing.T) {
	installed, latest := allTools()
	s := newSandbox(t, installed, latest)
	if code, out, errs := runCmd("update", "--json"); code != 2 || out != "" || errs != "rhythm: --json works with --check only\n" {
		t.Errorf("exit %d\n%s%s", code, out, errs)
	}
	if len(s.api.requests()) != 0 {
		t.Error("a usage error asked the API")
	}
}

func TestListJSON(t *testing.T) {
	s := listSandbox(t)
	code, out, errs := runManage("list", "--json")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	golden(t, "list.golden.json", out, s.dir, "$SANDBOX")
	var rep listReport
	objs := decodeStrict(t, out, &rep, "tools")
	if rep.Schema != "rhythm-list/v1" || len(rep.Tools) != 3 || !rep.Tools[2].ThirdParty {
		t.Errorf("%+v", rep)
	}
	requireKeys(t, objs, "name", "binary", "version", "latest", "signed", "path", "third_party", "managed_by", "error")
}

func TestDoctorJSON(t *testing.T) {
	s := newSandbox(t, map[string]string{"rhythm-tuner": "0.3.1"}, nil)
	fakeSelf(t, s.stub(t, "rhythm", "dev", ""), "dev")
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	env := fakeDoctorEnv(t, map[string]string{"RHYTHM_GITHUB_API": srv.URL, "GITHUB_TOKEN": "ghp_never_shown"})
	env.http = srv.Client()
	useDoctorEnv(t, env)

	code, out, errs := runManage("doctor", "--json")
	if code != 0 || errs != "" {
		t.Errorf("exit %d\n%s", code, errs)
	}
	golden(t, "doctor.golden.json", out, s.dir, "$SANDBOX", srv.URL, "$API", hostOf(srv.URL), "$HOST")
	var rep doctorReport
	requireKeys(t, decodeStrict(t, out, &rep, "checks"), "id", "status", "message")
	if rep.Schema != "rhythm-doctor/v1" || len(rep.Checks) == 0 || strings.Contains(out, "ghp_") {
		t.Errorf("%+v", rep)
	}

	// A failed check exits 1, with the JSON still whole.
	env.http = &http.Client{}
	env.getenv = func(k string) string {
		if k == "RHYTHM_GITHUB_API" {
			return closedPort(t)
		}
		return ""
	}
	useDoctorEnv(t, env)
	code, out, _ = runManage("doctor", "--json")
	if err := json.Unmarshal([]byte(out), &rep); code != 1 || err != nil || rep.Checks[len(rep.Checks)-1].Status != statusFail {
		t.Errorf("exit %d, %v\n%s", code, err, out)
	}
}
