# -*- coding: utf-8 -*-
"""Word-level diff for display, ported from the desktop app (app/web/js/diff.js).

English is compared word by word, Chinese character by character, punctuation on its own.
Whitespace is ignored when comparing and put back as it was when rendering.

1. Align the two token lists (difflib, no junk heuristic).
2. Semantic cleanup, as in the app: an unchanged run sitting between two changes and shorter than
   half of the changes on both sides counts as changed. Otherwise a deep rewrite gets shredded into
   confetti by small words ("the", "，", "的") that happen to line up.
3. A change that is only one or two punctuation marks is not marked.
"""
from __future__ import annotations

import difflib
import html
import re
import unicodedata

_CJK = "㐀-䶿一-鿿豈-﫿぀-ヿ가-힯"
TOKEN_RE = re.compile(
    r"\d+(?:[.,:/]\d+)+"            # 1,250 / 10:30 / 3.5
    rf"|[{_CJK}]"                   # one CJK character
    r"|[^\W_]+(?:['’][^\W_]+)*"  # a word (letters/digits), with inner apostrophes
    r"|\s+"
    r"|\S"
)
_CJK_ONE = re.compile(rf"^[{_CJK}]$")


def _is_punct(t: str) -> bool:
    return all(unicodedata.category(c)[0] in "PS" for c in t)


def _norm(t: str) -> str:
    t = unicodedata.normalize("NFKC", t)
    return t.replace("’", "'").replace("‘", "'").replace("“", '"').replace("”", '"')


def tokenize(s: str):
    """[(text, is_ws)] plus the content tokens [(norm, weight)]."""
    toks, content = [], []
    for m in TOKEN_RE.finditer(s or ""):
        t = m.group(0)
        if t.isspace():
            toks.append((t, True))
        else:
            toks.append((t, False))
            content.append((_norm(t), 2 if _CJK_ONE.match(t) else len(t)))
    return toks, content


def _ops(a, b):
    """[(op, i, j)]: op 0 = same (a[i], b[j]), -1 = deleted a[i], 1 = inserted b[j]."""
    sm = difflib.SequenceMatcher(None, [x for x, _ in a], [x for x, _ in b], autojunk=False)
    ops = []
    for tag, i1, i2, j1, j2 in sm.get_opcodes():
        if tag == "equal":
            ops += [(0, i1 + k, j1 + k) for k in range(i2 - i1)]
        else:
            ops += [(-1, i, -1) for i in range(i1, i2)]
            ops += [(1, -1, j) for j in range(j1, j2)]
    return ops


def _cleanup(ops, a, b):
    for _ in range(6):
        groups = []
        for op in ops:
            kind = 0 if op[0] == 0 else 1
            if not groups or groups[-1]["kind"] != kind:
                groups.append({"kind": kind, "ops": [], "len": 0, "del": 0, "ins": 0})
            g = groups[-1]
            g["ops"].append(op)
            if op[0] == 0:
                g["len"] += a[op[1]][1]
            elif op[0] == -1:
                g["del"] += a[op[1]][1]
            else:
                g["ins"] += b[op[2]][1]
        changed = False
        for k in range(1, len(groups) - 1):
            g = groups[k]
            if g["kind"] != 0:
                continue
            p, q = groups[k - 1], groups[k + 1]
            if g["len"] * 2 <= max(p["del"], p["ins"]) and g["len"] * 2 <= max(q["del"], q["ins"]):
                g["kind"] = 1
                g["ops"] = [x for _, i, j in g["ops"] for x in ((-1, i, -1), (1, -1, j))]
                changed = True
        if not changed:
            return ops
        ops = [op for g in groups for op in g["ops"]]
    return ops


def _quiet_punct(marks, toks):
    i = 0
    while i < len(marks):
        if not marks[i]:
            i += 1
            continue
        j = i
        while j < len(marks) and marks[j]:
            j += 1
        if j - i <= 2 and all(_is_punct(toks[k][0]) for k in range(i, j)):
            for k in range(i, j):
                marks[k] = 0
        i = j


def diff(draft: str, out: str):
    """Marks for both sides and the share of the rewrite (by weight) that is new wording."""
    at, a = tokenize(draft)
    bt, b = tokenize(out)
    ops = _cleanup(_ops(a, b), a, b)
    am, bm = [0] * len(a), [0] * len(b)
    for op, i, j in ops:
        if op == -1:
            am[i] = 1
        elif op == 1:
            bm[j] = 1
    _quiet_punct(am, a)
    _quiet_punct(bm, b)
    total = sum(w for _, w in b)
    new = sum(w for (_, w), m in zip(b, bm) if m)
    return (at, am), (bt, bm), (new / total if total else 0.0)


def render(toks, marks, cls: str) -> str:
    """HTML: marked content tokens wrapped in <span class=cls>. A space between two marked tokens
    joins them into one stroke; a line break never does."""
    content_idx, k = [], 0
    for t, ws in toks:
        content_idx.append(-1 if ws else k)
        if not ws:
            k += 1

    def marked(n):
        t, ws = toks[n]
        if not ws:
            return bool(marks[content_idx[n]])
        if "\n" in t:
            return False
        p, q = n - 1, n + 1
        while p >= 0 and toks[p][1]:
            p -= 1
        while q < len(toks) and toks[q][1]:
            q += 1
        return p >= 0 and q < len(toks) and bool(marks[content_idx[p]]) and bool(marks[content_idx[q]])

    out, span = [], False
    for n, (t, _) in enumerate(toks):
        m = marked(n)
        if m and not span:
            out.append(f'<span class="{cls}">')
            span = True
        elif not m and span:
            out.append("</span>")
            span = False
        out.append(html.escape(t, quote=False))
    if span:
        out.append("</span>")
    return "".join(out)


def marked_pair(draft: str, out: str):
    """(draft_html with .del marks, rewrite_html with .ins marks, share of new wording)."""
    (at, am), (bt, bm), ratio = diff(draft, out)
    return render(at, am, "del"), render(bt, bm, "ins"), ratio
