#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""markdown_guard.py — keep Markdown headings (and fenced code) through a rewrite.

Why (issue #1, 2026-09-20): the model drops `#`…`####` headings. Its training targets are
human-written texts, which almost never carry ATX headings, and AI detectors treat headings as a
strong machine signal — so the model learned to write prose without them. That is the intended
default. When you need the headings back, this module splits the draft at heading lines (and
around ``` fences), rewrites each prose block on its own, and re-inserts the headings and code
verbatim in their original order. Nothing in the model or prompt changes.

Cost: each section is rewritten without seeing the others (less cross-section flow), and a
document full of headings keeps a strong AI-looking skeleton, so expect a lower pass rate on
detectors than the default mode.
"""
import re

HEADING = re.compile(r'^\s{0,3}#{1,6}\s+\S')
FENCE = re.compile(r'^\s{0,3}(```|~~~)')
LIST_ITEM = re.compile(r'^(\s{0,3}(?:[-*+]|\d{1,3}[.)])\s+)(\S.*)$')
MIN_WORDS = 6   # a prose block shorter than this is passed through untouched (too little context to rewrite safely)


def split_blocks(text):
    """→ list of (kind, text): kind ∈ {'heading', 'code', 'list', 'prose'}. Order preserved, no text lost.
    A run of list lines is one 'list' block; each item is rewritten on its own so the bullets survive."""
    blocks, prose, code, fence = [], [], [], None
    for line in text.splitlines():
        if fence is not None:
            code.append(line)
            if FENCE.match(line) and line.strip().startswith(fence):
                blocks.append(('code', '\n'.join(code))); code, fence = [], None
            continue
        if FENCE.match(line):
            if prose: blocks.append(('prose', '\n'.join(prose))); prose = []
            fence = FENCE.match(line).group(1); code = [line]; continue
        if HEADING.match(line):
            if prose: blocks.append(('prose', '\n'.join(prose))); prose = []
            blocks.append(('heading', line.rstrip())); continue
        if LIST_ITEM.match(line):
            prose_had_gap = bool(prose)   # a blank line between two lists keeps them two lists
            if ''.join(prose).strip(): blocks.append(('prose', '\n'.join(prose)))
            prose = []
            if blocks and blocks[-1][0] == 'list' and not prose_had_gap: blocks[-1] = ('list', blocks[-1][1] + '\n' + line.rstrip())
            else: blocks.append(('list', line.rstrip()))
            continue
        prose.append(line)
    if fence is not None: blocks.append(('code', '\n'.join(code)))
    if prose: blocks.append(('prose', '\n'.join(prose)))
    return blocks


def rewrite_keeping_markdown(draft, rewrite):
    """rewrite(prose_text) -> rewritten prose. Headings and fenced code come back verbatim."""
    out = []
    for kind, body in split_blocks(draft):
        if kind == 'prose':
            if len(body.split()) < MIN_WORDS:
                if body.strip(): out.append(body.strip())
                continue
            out.append(rewrite(body.strip()).strip())
        elif kind == 'list':
            items = []
            for line in body.splitlines():
                m = LIST_ITEM.match(line)
                if m and len(m.group(2).split()) >= MIN_WORDS:
                    items.append(m.group(1) + ' '.join(rewrite(m.group(2)).split()))
                else:
                    items.append(line)
            out.append('\n'.join(items))
        else:
            out.append(body)
    return '\n\n'.join(b for b in out if b != '')


if __name__ == '__main__':
    import sys
    for kind, body in split_blocks(sys.stdin.read()):
        print(f'[{kind}] {body[:70]!r}')
