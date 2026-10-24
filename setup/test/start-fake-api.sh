#!/usr/bin/env bash
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

# Starts the fake release API for setup-rhythm-test.yml in the background
# and waits until it answers:
#
#   bash setup/test/start-fake-api.sh
#
# It builds the rhythm stand-in (scripts/test/stub, as 0.7.0 with every
# feature) and setup/internal/fakeapi, then serves them on the address in
# RHYTHM_GITHUB_API (default http://127.0.0.1:23000). The server's log goes
# to $RUNNER_TEMP/fakeapi.log (or the temp folder).
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
work=${RUNNER_TEMP:-${TMPDIR:-/tmp}}/setup-rhythm-fake
api=${RHYTHM_GITHUB_API:-http://127.0.0.1:23000}
addr=${api#http://}
mkdir -p "$work"
exe=
case $(uname -s) in MINGW* | MSYS* | CYGWIN*) exe=.exe ;; esac

export GOWORK=off
go -C "$root" build -o "$work/rhythm$exe" \
	-ldflags "-X main.version=0.7.0 -X main.features=dry-run,verify,defaults,pins,locked,skip-signature" ./scripts/test/stub
go -C "$root" build -o "$work/fakeapi$exe" ./setup/internal/fakeapi
nohup "$work/fakeapi$exe" -stub "$work/rhythm$exe" -addr "$addr" >"$work/../fakeapi.log" 2>&1 &

for _ in $(seq 1 50); do
	if curl -fsS -o /dev/null "$api/repos/RhythmShahriar/rhythm/releases/latest" 2>/dev/null; then
		echo "fake release API up at $api"
		exit 0
	fi
	sleep 0.2
done
cat "$work/../fakeapi.log" >&2 || true
echo "the fake release API didn't start at $api" >&2
exit 1
