#!/usr/bin/env python3
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

"""Checks the wiki before it's published: every internal link points at a
page that exists (and at a heading that exists, for #anchors). Every
page must be reachable from the sidebar. Usage: check-wiki.py <wiki-dir>"""
import os
import re
import sys

LINK = re.compile(r'(?<!!)\[[^\]]*\]\(([^)\s]+)\)')
HREF = re.compile(r'href="([^"]+)"')
HEADING = re.compile(r'^(#{1,6})\s+(.*?)\s*#*\s*$')
FENCE = re.compile(r'^\s*(```|~~~)')


def slug(text):
    # GitHub's anchor rules: lowercase, drop punctuation, spaces to dashes.
    text = re.sub(r'<[^>]+>', '', text)
    # Links keep their text; *, ` and ~ go. Underscores stay: GitHub keeps
    # them, so "min_tuner" anchors as min_tuner.
    text = re.sub(r'[`*~]|\[([^\]]*)\]\([^)]*\)', lambda m: m.group(1) or '', text)
    text = text.strip().lower()
    text = re.sub(r'[^\w\- ]', '', text)
    return text.replace(' ', '-')


def headings(path):
    seen, anchors, fenced = {}, set(), False
    with open(path, encoding='utf-8') as f:
        for line in f:
            if FENCE.match(line):
                fenced = not fenced
                continue
            m = None if fenced else HEADING.match(line)
            if m:
                base = slug(m.group(2))
                n = seen.get(base, 0)
                seen[base] = n + 1
                anchors.add(base if n == 0 else f'{base}-{n}')
    return anchors


def links(path):
    fenced = False
    with open(path, encoding='utf-8') as f:
        for no, line in enumerate(f, 1):
            if FENCE.match(line):
                fenced = not fenced
                continue
            if fenced:
                continue
            line = re.sub(r'`[^`]*`', '', line)  # code spans aren't links
            for target in LINK.findall(line) + HREF.findall(line):
                yield no, target


def main(root):
    pages = {f[:-3] for f in os.listdir(root) if f.endswith('.md')}
    anchors = {p: headings(os.path.join(root, p + '.md')) for p in pages}
    errors = []
    for page in sorted(pages):
        for no, target in links(os.path.join(root, page + '.md')):
            if re.match(r'^[a-z][a-z0-9+.-]*:', target) or target.startswith(('/', 'mailto:')):
                continue  # external
            name, _, anchor = target.partition('#')
            name = name or page
            if name.endswith('.md'):
                errors.append(f'{page}.md:{no}: link to {target!r}: drop the .md, wiki links use the page name')
                continue
            if name not in pages:
                errors.append(f'{page}.md:{no}: link to missing page {name!r}')
            elif anchor and anchor not in anchors[name]:
                errors.append(f'{page}.md:{no}: no heading #{anchor} in {name}')
    if '_Sidebar' in pages:
        listed = {t.partition('#')[0] for _, t in links(os.path.join(root, '_Sidebar.md'))}
        for page in sorted(pages - listed - {'_Sidebar', '_Footer'}):
            errors.append(f'{page}.md: not in _Sidebar.md, so readers can only find it by search')
    for e in errors:
        print(e)
    print(f'{len(pages)} pages, {len(errors)} problems')
    return 1 if errors else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else 'wiki'))
