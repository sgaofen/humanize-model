---
title: humanizer
emoji: ✍️
colorFrom: gray
colorTo: green
sdk: gradio
sdk_version: 6.29.0
app_file: app.py
pinned: false
license: apache-2.0
short_description: Rewrite AI drafts to read like a person; facts kept. 12B
models:
  - jialinyyzz/humanizer
---

# humanizer: demo

A 12B model that rewrites AI-written drafts (emails, essays, reports, forum posts; English and Chinese)
so they read like a person wrote them. It is trained to keep every number, unit, date, name and quote,
and to add nothing. No AI detector was used anywhere in training.

This Space runs the bf16 weights from [`jialinyyzz/humanizer`](https://huggingface.co/jialinyyzz/humanizer)
on ZeroGPU, one draft per run (up to about 700 English words or 1,200 Chinese characters). Real
before/after pairs from the held-out evaluation set are on the page and need no GPU.

**Run it on your own computer:** the desktop app for macOS (Apple silicon) and Windows is on
[GitHub Releases](https://github.com/sgaofen/humanize-model/releases/latest); the command-line tool
`hz` and llama.cpp, MLX, transformers and vLLM steps are in
[USAGE.md](https://github.com/sgaofen/humanize-model/blob/main/docs/USAGE.md)
([中文](https://github.com/sgaofen/humanize-model/blob/main/docs/USAGE.zh.md)) and
[AGENTS.md](https://github.com/sgaofen/humanize-model/blob/main/AGENTS.md).

**Measured, not promised:** on our 210 English evaluation drafts, Originality.ai (AI Allowance 0%,
2026-10-01) flagged 26; a strict LLM judge found a severe fact error in 51 of 420 English outputs.
Chinese is weaker (135 of 204 outputs passed the fact judge). Proofread every number, date and name.

**API:** `gradio_client` → `Client("jialinyyzz/humanizer").predict(draft, api_name="/humanize")` returns
the rewrite plus copy rate and any numbers missing from it.

**Privacy:** the Space keeps no text. It logs counts only (lengths, timing, copy rate), never the draft,
the output or your IP.

Source of this Space: [`space/`](https://github.com/sgaofen/humanize-model/tree/main/space) in the GitHub repo.

---

把 AI 写的草稿改成读起来像人写的，训练目标是数字、单位、日期、人名、引语原样保留。中英文都行。训练全程没有用任何 AI 检测器。
这里可以在线试一篇；想装到自己电脑上，下载 [App（macOS / Windows）](https://github.com/sgaofen/humanize-model/releases/latest)，
或者看 [不用 App 怎么用](https://github.com/sgaofen/humanize-model/blob/main/docs/USAGE.zh.md)。
