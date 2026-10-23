# rhythm

One CLI for every Rhythm tool. Useful ideas. Working systems.

```sh
curl -fsSL rythm.sh/install | sh
rhythm tuner scan example.com
```

| Tool | What it does |
|---|---|
| [tuner](https://github.com/RhythmShahriar/tuner) | WordPress & PHP security scanner |

`rhythm <tool>` runs the `rhythm-<tool>` binary, the same extension model as `git` and `gh`.

## Install

```sh
curl -fsSL rythm.sh/install | sh
```

This installs `rhythm` and Tuner to `~/.local/bin` (`/usr/local/bin` as root), verifying every download against the release's `checksums.txt`. Set `RHYTHM_INSTALL_DIR`, `RHYTHM_VERSION`, or `RHYTHM_TOOLS=""` to change that.

## Use

```sh
rhythm tuner scan example.com    # or: rhythm t scan example.com
rhythm install tuner             # add a tool
rhythm update                    # update rhythm, tools and their data
rhythm update --check            # only report available updates
```

Release checksums are signed with keyless [cosign](https://docs.sigstore.dev/):

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/RhythmShahriar/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
```

## Development

```sh
make hooks   # Conventional Commits check
make check   # lint + tests
```

`kit/` is the shared base (theme, output, config, update) imported by every tool.

## License

[AGPL-3.0](LICENSE). Copyright 2026 Rhythm Shahriar.
