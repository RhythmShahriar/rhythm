How well does Sightread catch a bad update, how quiet does it stay on a good one and does it ever run what it reads? This page describes the benchmark: a testbed of 23 bad updates, 15 good ones and 15 environment scenarios, how each is run and judged and the results. **Sightread caught 23 of 23 bad updates, raised no false alarm on 15 good ones or in 150 control runs and passed 14 of 15 environment scenarios**; the one left (e13) needs a Windows machine and wasn't run.

> [!NOTE]
> We wrote Sightread and the testbed, so read this as a regression suite with a scoreboard, not an independent review. Every bad scenario is something we built Sightread to catch, modelled on real incidents (the 2026 WordPress.org plugin backdoor, Shai-Hulud). Every good scenario is a benign update that must stay quiet. The fixtures are not real packages. A real attacker who reads our detectors can write code they miss; Sightread lowers the risk of an update, it does not prove an update is safe.

## The testbed

The testbed lives in the Sightread repository, in [`testbed/`](https://github.com/RhythmShahriar/sightread/tree/master/testbed). Everything runs on 127.0.0.1:

| Service | Port | What it is |
|---|---|---|
| Fixture registry | 18900-18904, 18908 | one Go server playing WordPress.org (info API, zips, checksums, SVN), Packagist, GitHub (zipball redirect to codeload on a second host name), npm (packuments, tarballs, signing keys, signatures) and OSV |
| Canary | 18905 | every bad payload would connect here and write a file if its code ever ran |
| Webhook receiver | 18906 | receives `watch` notifications |
| WordPress | 18907 | WordPress 6.8.3 in Docker on a network with no route out, for `--soundcheck` |
| Probes and fronts | 23300-23339 | a stalled body, an endless body, 429 then 200, a redirect to a cloud metadata address, self-signed TLS |
| Proxy guard | 23399 | `HTTPS_PROXY` points here; it closes every connection and counts it |

The registry builds every fixture archive at start from `testbed/packages/<id>/{old,new}` and signs npm versions with a key generated at start. Fake secrets are assembled at run time. Sightread runs with every `SIGHTREAD_*` base pointed at the registry, so nothing reaches a real registry. The proxy guard counts any request that forgets a base. Each scenario has a good state and a bad state; `testbed/toggle.sh b01 good` withholds the bad candidate on the next request, with no restart.

## Scenarios

`testbed/scenarios.yaml` says what each scenario updates, the command that reviews it and what counts as caught: the verdict, the exit code and every listed signal at or above its severity. A good scenario must report nothing at or above medium beyond what it lists; g15 must report exactly its one signal.

| | Bad (must be caught) | Good (must stay quiet) |
|---|---|---|
| WordPress plugins | b01 owner change and a cron that fetches and runs remote code, b02 sends admin email and users to a new host, b03 creates and hides an administrator, b04 gives subscribers `manage_options`, b05 layered decode that runs built code, b06 unserializes a cookie, b07 writes a mu-plugin and `.htaccess`, b08 PHP hidden in an image, b17 tampered archive, b18 same version with new bytes after an allow, b21 Essential Plugin replay, b22 Gravity Forms pattern from a vendor zip | g01 large feature release with real permission callbacks, g02 rebuilt minified bundle, g03 thirty translation files, g04 bundled Guzzle bump, g05 documented daily cron, g06 a contributor added, g08 line endings only, g15 official WordPress.org takeover |
| WordPress themes | b09 SEO spam fetched for bots only | g07 CSS and templates only |
| npm | b10 postinstall that downloads and runs a script, b11 Shai-Hulud style token theft, b12 all maintainers replaced, b13 typosquat with an install script, b14 lock entry moved off the registry, b20 axios pattern (fresh dependency, no provenance, no tag), b23 Trojan Source plus an obfuscated blob | g09 plain patch, g10 unchanged node-gyp install, g11 popular dependencies added |
| Composer | b15 `autoload.files` that fetches and runs code, b16 becomes a Composer plugin, b19 tag moved to another commit | g12 GitHub dist with no shasum, g13 minor with a new class |
| Whole site | | g14 a site with both lockfiles, every package on its good candidate |

Environment scenarios: e01 registry down, e02 429 then success, e03 stalled body, e04 zip bomb, e05 hostile archive entries, e06 a cached blob changed on disk, e07 two runs sharing one cache, e08 Ctrl+C during a fetch, e09 offline with a warm cache, e10 read-only HOME, e11 private names next to public namesakes, e12 a version unpublished from npm, e13 a Windows run, e14 escape sequences in names and changelogs, e15 `--soundcheck` on the Docker WordPress.

## Results

Run on 2026-10-28 with `make -C testbed bench` on the integration build of 1.0 (linux/amd64, 16 CPUs, Go 1.27.3), the Docker WordPress up and Soundcheck 1.2.2 installed, no compared tools. It took 3m39s.

| 1.0 target | Result | Detail |
|---|---|---|
| Every bad scenario caught | met | 23 of 23 caught, none pending |
| Zero false alarms on good scenarios | met | 15 of 15 clean |
| Every run wrote a report | met | 0 without a report |
| Exit codes, verdicts and Not checked as expected | met | none differ |
| Zero false alarms in the control pass | met | 10 rounds, 150 runs, 0 false alarms, no outcome changed between rounds |
| Zero canary, egress, outside-write, unmasked-key and forwarded-token events | met | all 0 |
| Every environment scenario passes | met | failed: none; not run here: e13 |
| Cold and warm runs agree | met | 38 of 38 scenarios |
| p01-large under 10 s warm and 512 MB | met | 4,021 files, 40 MB: warm 2.9 s, peak 105 MB (cold 2.0 s, 87 MB) |
| Full benchmark under 10 minutes | met | 3m39s |

Every bad scenario was caught with the signals it lists, found as nothing else and missed by nothing:

| Scenario | Signals reported |
|---|---|
| b01 | changelog-mismatch, cron-new-hook, owner-change, remote-fetch-exec |
| b02 | data-exfil, outbound-new-domain |
| b03 | admin-user-create, admin-user-hidden |
| b04 | capability-change |
| b05 | decode-exec |
| b06 | input-unserialize |
| b07 | write-outside-package |
| b08 | code-in-asset |
| b09 | bot-cloaking, outbound-new-domain |
| b10 | npm-install-script-new, remote-fetch-exec |
| b11 | credential-access, npm-install-script-new |
| b12 | outbound-new-domain, owner-change |
| b13 | dependency-new, npm-install-script-new, typosquat |
| b14 | lockfile-source-change |
| b15 | composer-autoload-files, remote-fetch-exec |
| b16 | composer-plugin-new |
| b17 | integrity-mismatch |
| b18 | approved-version-changed |
| b19 | version-republished |
| b20 | dependency-new, fresh-release, git-tag-missing, provenance-dropped |
| b21 | changelog-mismatch, eval-dynamic, input-unserialize, owner-change, rest-open-route |
| b22 | admin-user-create, outbound-new-domain, update-suppression |
| b23 | obfuscation, unicode-hidden |

## Safety

Every bad payload targets the canary. HOME and the project fixtures are walked before and after every run. Every output and report is searched for the fake keys. Codeload answers on a second host name, so a `GITHUB_TOKEN` sent across the zipball redirect would be counted.

| Counter | Count | Must be |
|---|---|---|
| Canary connections | 0 | 0 |
| Canary files | 0 | 0 |
| Egress attempts | 0 | 0 |
| Files written outside the work folders | 0 | 0 |
| Fake keys printed or written unmasked | 0 | 0 |
| Codeload requests carrying GITHUB_TOKEN | 0 | 0 |

## --soundcheck end to end

e15 runs against the sealed Docker WordPress with a Roadie stand-in (Roadie ships separately). The applying case uses the real `rhythm-soundcheck`; every refusal must stop before any change, with the container's plugin and the host's files byte-identical.

| Case | Result | Notes |
|---|---|---|
| update lands in the container only | pass | exit 0, e15-soundcheck 1.0.0 to 1.1.0 |
| refused: HOLD | pass | exit 1, b01-widget stays 2.6.7 |
| refused: production | pass | exit 2 |
| refused: Roadie failed | pass | exit 3 |
| refused: Roadie check older than the copy | pass | exit 3 |
| refused: copy not sealed | pass | exit 3 |
| refused: rhythm-roadie missing | pass | exit 3 |
| refused: remote `--wp` prefix | pass | exit 2 |

Soundcheck's Chrome tried 197 outside calls to Google services during the applying case. Its own proxy guard refused every one; they aren't Sightread's and aren't counted as its egress.

## Speed and memory

Wall time and peak memory of the `rhythm-sightread` process. Cold is an empty cache, warm the second run on the same cache. Every scored scenario reviewed in 0.02 to 0.14 s cold, apart from g14 (a whole site with both lockfiles, 0.72 s). Warm runs took 0.02 to 0.05 s, at a peak of 87 MB.

| Target (from the spec) | How it is measured |
|---|---|
| Largest fixture under 10 s warm and 512 MB | p01-large above: 2.9 s, 105 MB |
| PHP lexing at least 50 MB/s per core | `make perf` in the repository |
| One plugin side under 2 s | `make perf` |
| The demo under 3 s | `make perf` |
| Real releases: one 5 MB plugin under 10 s cold, `all` on 30 plugins under 60 s | owner-run with network, not in this board |

## Compared tools

The benchmark can run `composer audit`, `npm audit` and GuardDog on the same fixture states, plus Soundcheck before and after as the stand-in for a visual-only update check. They weren't run here. Audits know only published advisories, so they miss a fresh malicious update by design, not by defect; a visual check can't see a backdoor that changes nothing on the page.

## Known gaps

What this board does not cover:

- Logic that waits for a date, a domain or a visitor count; code fetched at run time from a server that serves clean code to scanners.
- Obfuscation beyond the layered decodes in b05 and b23, compiled assets and binaries: `unanalysed` is reported, not what they do.
- Evasions written against the published detectors.
- Owner changes that keep the same account; sales WordPress.org doesn't show.
- Typosquats of niche packages that are not on the popular-names list.
- Case and Unicode collisions on macOS and Windows file systems.
- Windows-only behaviour (e13): the nightly `bench-windows` job runs it on windows-latest.
- Real releases: the fixtures are not real packages. A separate owner-run section of `docs/benchmark.md` measures false alarms on 50 popular WordPress.org plugins and 50 npm packages pinned by hash, with network access; it isn't part of `make bench` or CI.

## Run it yourself

```sh
git clone https://github.com/RhythmShahriar/sightread && cd sightread
make -C testbed bench                          # every scenario; writes docs/benchmark.md
make -C testbed bench BENCH="-only b01,g05,env"
make -C testbed up-wp                          # the Docker WordPress for e15
make -C testbed reproduce                      # the benchmark twice, outcomes diffed
make perf                                      # the analyzer and lexer speed targets
```

It needs Go. Docker with Compose v2 is needed only for e15. The full generated page, scenario by scenario, is [`docs/benchmark.md`](https://github.com/RhythmShahriar/sightread/blob/master/docs/benchmark.md).

---
<sub>← [FAQ and honest limits](Sightread-FAQ) · [Home](Home) · [Responsible use](Responsible-Use) →</sub>
