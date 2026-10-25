# Roadmap

What we plan for rhythm and the shared kit. Plans change as we learn, so treat this as a direction, not a promise.

Have an idea or a use case we missed? Start a thread in [Discussions: Ideas](https://github.com/RhythmShahriar/rhythm/discussions/categories/ideas). Want to help now? Pick a [good first issue](https://github.com/RhythmShahriar/rhythm/labels/good%20first%20issue).

## Now

- **Update prompt.** After a run, ask "update now? [y/N]" when a newer release is out, in rhythm and every tool.
- **One CLI contract for every tool.** Tuner, Soundcheck and Echo move onto `kit/cli.Main`: `version --json`, the shared exit codes, signal handling and the update notice.
- **Signing identity cut-over.** The tool repos stop calling the shared workflows at `@master`, so a release can be checked against an exact workflow and tag.

## Next

- **Package managers.** Publish to Homebrew, Scoop and winget from every release.
- **Tool registry.** A `tools.json` that lists the tools, so new ones appear in `rhythm install` and `rhythm update` without a new rhythm release.
- **More shared kit.** Store run folders and the production guard move into the kit (`kit/store`, `kit/guard`). Every tool uses `kit/httpx` for host-scoped headers and redirects.
- **Shared CI checks.** A lint step that enforces a positive and a negative fixture for every Tuner check, Echo recognizer and Soundcheck rule.

## Later

- **Update night.** One command that scans with Tuner, seals with Echo and lets Soundcheck find the plugin update that broke the site.
- **`rhythm mcp`.** One MCP server for agents doing site updates: scan, snapshot, compare and the latest message.
- **One fleet file.** A single `sites.yaml` drives Tuner, Soundcheck and Echo, with one report per client.
- **Signed Windows and macOS binaries.** Authenticode and notarization.
