---
license: apache-2.0
base_model: google/gemma-4-12B
base_model_relation: finetune
language:
- en
- zh
pipeline_tag: text-generation
library_name: transformers
tags:
- humanizer
- text-rewriting
- rewriting
- paraphrase
- style-transfer
- gguf
- llama.cpp
- mlx
- gemma4
---

<img src="assets/banner-en.png" alt="humanizer: rewrites AI drafts so they read like a person wrote them" width="100%">

<img src="assets/app-showcase-en.png" alt="The humanizer app running the 12B model locally" width="100%">

# humanizer

**A 12B model that rewrites AI-written drafts (emails, essays, reports, forum posts; English and Chinese) so they read like a person wrote them.** It is trained to keep every number, unit, date, name and quote, and to add nothing. It runs locally. No AI detector was used anywhere in training.

**[Usage without the app](USAGE.md)** · [不用 App 怎么用](USAGE.zh.md) · [AGENTS.md (for AI agents)](AGENTS.md) · [GitHub](https://github.com/sgaofen/humanize-model) · [Desktop app (macOS, Windows)](https://github.com/sgaofen/humanize-model/releases/latest) · [Install guide](https://github.com/sgaofen/humanize-model/blob/main/docs/INSTALL.md) · [中文说明](https://github.com/sgaofen/humanize-model/blob/main/README.zh.md)

> **Setting this up with an AI agent?** Point it at [AGENTS.md](AGENTS.md): exact files, server command, prompt byte for byte, and a self-test.

## Quick start

**App:** download the `.dmg` (Mac with Apple silicon) or the Windows installer from [Releases](https://github.com/sgaofen/humanize-model/releases/latest). On first run it picks a model size for your memory and downloads it once; after that it works offline.

**Without the app:** pick a file below, then follow [Usage without the app](#usage-without-the-app). The full guide, with a batch script, long documents, Chinese and troubleshooting, is **[USAGE.md](USAGE.md)** ([中文](USAGE.zh.md)).

## Files

| File | Size | For |
|---|---|---|
| `humanizer-12b-Q8_0.gguf` | 12,669,627,840 bytes (about 12.7 GB) | 32 GB of memory or more. Recommended. |
| `humanizer-12b-Q6_K.gguf` | 10,029,797,088 bytes (about 10.0 GB) | 16 GB of memory. |
| `humanizer-12b-Q4_K_M.gguf` | about 7.6 GB | *Coming soon*: released only after it passes the fact judge. |
| `model.safetensors` + `config.json`, `generation_config.json`, `tokenizer.json`, `tokenizer_config.json` | about 24 GB (bf16) | transformers, vLLM, converting to MLX. |
| `prompt_format.json` | tiny | The instruction and separator, verbatim. |
| `lite/` | `humanizer-lite-Q8_0.gguf` about 8.0 GB · `humanizer-lite-Q6_K.gguf` about 6.2 GB · `humanizer-lite-bf16.gguf` about 14.9 GB · safetensors (4 shards) about 15.9 GB, with config, tokenizer and `prompt_format.json` | The previous E4B release (formerly `jialinyyzz/humanizer-gemma-4-e4b`), for 8 GB machines. Same prompt format. |

Q6_K and Q4_K_M are imatrix-calibrated on our own rewriting data, with the embeddings and output layer kept at 8-bit. Difference from bf16, measured on 104 drafts and their rewrites from the evaluation set (no overlap with the calibration data):

| File | Mean KL vs. bf16 | Top token same as bf16 | Perplexity |
|---|---|---|---|
| Q8_0 | 0.0017 | 98.4% | +0.2% |
| Q6_K | 0.0033 | 97.8% | +0.5% |
| Q4_K_M | 0.0214 | 93.9% | +2.3% |

The Q4_K_M loss is clearly larger, so it waits for the fact judge. sha256 checksums: <!-- TBD: sha256 after upload --> *coming soon*.

## Prompt format

**Text completion, not chat.** No system prompt, no chat template, no turn markers. Send exactly this text and let the model continue:

```
Rewrite the text below so it reads like a person wrote it, not a language model.

Reorganize it as you see fit. Vary sentence length on purpose. Cut hedging,
throat-clearing, and any sentence that only announces what comes next.
Prefer the concrete word over the abstract one. It is fine to sound uneven.

Every fact, number, unit, date, name and quotation must survive unchanged.

<YOUR DRAFT, with leading and trailing whitespace removed>

### Rewritten:

```

- `prompt = instr + "\n\n" + draft.strip() + "\n\n### Rewritten:\n\n"`; both strings are in `prompt_format.json`. Reproduce it byte for byte: the first 16 hex characters of `sha256(prompt for the draft "X")` must be `cc51d66b4c593fbe`.
- Use the same English instruction for Chinese drafts.
- **Stop on EOS only.** No stop strings, especially not `"###"`.
- **Sampling: temperature 1.0, top-p 0.95, nothing else** (top-k 0, min-p 0, repetition penalty 1.0). llama.cpp defaults to top-k 40 and min-p 0.05, and the bundled `generation_config.json` sets top-k 64, so switch them off explicitly.
- Context 8192 tokens for instruction + draft + rewrite. Split long documents at paragraph breaks ([USAGE.md](USAGE.md#10-long-documents)).

## Usage without the app

All snippets below build the prompt from `prompt_format.json` and use the sampling above. More detail for each runtime, a script that rewrites a whole folder, and a troubleshooting table: [USAGE.md](USAGE.md).

### llama.cpp (recommended; macOS, Windows, Linux)

```bash
brew install llama.cpp            # or: winget install llama.cpp / a zip from github.com/ggml-org/llama.cpp/releases
pip install -U "huggingface_hub[cli]"
hf download jialinyyzz/humanizer humanizer-12b-Q8_0.gguf prompt_format.json --local-dir ./humanizer-model
#   16 GB machine: humanizer-12b-Q6_K.gguf instead
llama-server -m ./humanizer-model/humanizer-12b-Q8_0.gguf -c 8192 -np 1 -ngl 99 --host 127.0.0.1 --port 8080
```

```python
import json, urllib.request

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))

def humanize(draft: str, url: str = "http://127.0.0.1:8080") -> str:
    body = {"prompt": PF["instr"] + "\n\n" + draft.strip() + PF["sep"],
            "temperature": 1.0, "top_p": 0.95, "top_k": 0, "min_p": 0, "repeat_penalty": 1.0,
            "n_predict": 2048}                    # no "stop": the model ends at EOS
    req = urllib.request.Request(url + "/completion", json.dumps(body).encode("utf-8"),
                                 {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=900) as r:
        return json.load(r)["content"].strip()

print(humanize(open("draft.txt", encoding="utf-8").read()))
```

Use `/completion`, not `/v1/chat/completions` (that one applies the chat template). With curl and jq:

```bash
jq -n --rawfile d draft.txt --slurpfile f humanizer-model/prompt_format.json \
  '{prompt: ($f[0].instr + "\n\n" + ($d | sub("^\\s+"; "") | sub("\\s+$"; "")) + $f[0].sep),
    temperature: 1.0, top_p: 0.95, top_k: 0, min_p: 0, repeat_penalty: 1.0, n_predict: 2048}' \
| curl -s http://127.0.0.1:8080/completion -d @- | jq -r .content
```

### MLX (Apple silicon, mlx-lm 0.32 or newer)

```bash
pip install -U "mlx-lm>=0.32" huggingface_hub
mlx_lm.convert --hf-path jialinyyzz/humanizer --mlx-path humanizer-mlx-8bit -q --q-bits 8 --q-group-size 64
```

```python
import json
from huggingface_hub import hf_hub_download
from mlx_lm import load, generate
from mlx_lm.sample_utils import make_sampler

PF = json.load(open(hf_hub_download("jialinyyzz/humanizer", "prompt_format.json"), encoding="utf-8"))
model, tok = load("humanizer-mlx-8bit")
draft = open("draft.txt", encoding="utf-8").read()
print(generate(model, tok, prompt=PF["instr"] + "\n\n" + draft.strip() + PF["sep"], max_tokens=2048,
               sampler=make_sampler(temp=1.0, top_p=0.95)).strip())
```

Pass a plain string; never apply the chat template (the `mlx_lm.generate` CLI needs `--ignore-chat-template`).

### transformers (CUDA)

```python
import json, torch
from huggingface_hub import hf_hub_download
from transformers import AutoModelForCausalLM, AutoTokenizer

repo = "jialinyyzz/humanizer"
PF = json.load(open(hf_hub_download(repo, "prompt_format.json"), encoding="utf-8"))
tok = AutoTokenizer.from_pretrained(repo)
model = AutoModelForCausalLM.from_pretrained(repo, dtype=torch.bfloat16, device_map="auto")

draft = open("draft.txt", encoding="utf-8").read()
ids = tok(PF["instr"] + "\n\n" + draft.strip() + PF["sep"], return_tensors="pt").to(model.device)
out = model.generate(**ids, do_sample=True, temperature=1.0, top_p=0.95,
                     top_k=0,                    # switches off the top-k 64 in generation_config.json
                     max_new_tokens=2048)
print(tok.decode(out[0, ids["input_ids"].shape[1]:], skip_special_tokens=True).strip())
```

The bf16 weights are about 24 GB. Saved with transformers 5.14.1; on 4.x write `torch_dtype=` instead of `dtype=`.

### vLLM

```python
import json
from huggingface_hub import hf_hub_download
from vllm import LLM, SamplingParams

repo = "jialinyyzz/humanizer"
PF = json.load(open(hf_hub_download(repo, "prompt_format.json"), encoding="utf-8"))
llm = LLM(model=repo, dtype="bfloat16", max_model_len=8192,
          limit_mm_per_prompt={"image": 0, "audio": 0, "video": 0})   # text only
params = SamplingParams(temperature=1.0, top_p=0.95, top_k=-1, min_p=0.0,
                        repetition_penalty=1.0, max_tokens=2048)      # top_k=-1: off

drafts = [open(p, encoding="utf-8").read() for p in ["draft1.txt", "draft2.txt"]]
for r in llm.generate([PF["instr"] + "\n\n" + d.strip() + PF["sep"] for d in drafts], params):
    print(r.outputs[0].text.strip(), "\n---")
```

This mirrors how our evaluation outputs were generated. As a server: `vllm serve jialinyyzz/humanizer --dtype bfloat16 --max-model-len 8192 --generation-config vllm`, then `/v1/completions` (never `/v1/chat/completions`) with the same parameters; `--generation-config vllm` keeps the top-k 64 from `generation_config.json` out of the defaults.

### Ollama

Ollama applies a chat template unless you pass the prompt through untouched and call it in raw mode. Needs an Ollama version that supports Gemma 4 models; untested by us.

```
FROM ./humanizer-12b-Q8_0.gguf
TEMPLATE """{{ .Prompt }}"""
PARAMETER temperature 1.0
PARAMETER top_p 0.95
PARAMETER top_k 0
PARAMETER min_p 0
PARAMETER repeat_penalty 1.0
PARAMETER num_ctx 8192
PARAMETER num_predict 2048
```

```bash
ollama create humanizer -f Modelfile
jq -n --rawfile d draft.txt --slurpfile f humanizer-model/prompt_format.json \
  '{model: "humanizer", raw: true, stream: false,
    prompt: ($f[0].instr + "\n\n" + ($d | sub("^\\s+"; "") | sub("\\s+$"; "")) + $f[0].sep),
    options: {temperature: 1.0, top_p: 0.95, top_k: 0, min_p: 0, repeat_penalty: 1.0,
              num_ctx: 8192, num_predict: 2048}}' \
| curl -s http://127.0.0.1:11434/api/generate -d @- | jq -r .response
```

Don't use interactive `ollama run` or `/api/chat`.

### LM Studio

Untested by us. Load the GGUF with an 8192-token context; in the model's sampling settings set Temperature 1.0, Top P 0.95, Top K 0, Min P 0, Repeat Penalty 1.0 and remove stop strings; start the local server and send the full prompt to the **text-completion endpoint `/v1/completions`**:

```python
import json, urllib.request

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))
draft = open("draft.txt", encoding="utf-8").read()
body = {"model": "humanizer-12b-q8_0",           # replace with the model id shown in LM Studio
        "prompt": PF["instr"] + "\n\n" + draft.strip() + PF["sep"],
        "temperature": 1.0, "top_p": 0.95, "top_k": 0, "min_p": 0, "repeat_penalty": 1.0, "max_tokens": 2048}
req = urllib.request.Request("http://127.0.0.1:1234/v1/completions", json.dumps(body).encode("utf-8"),
                             {"Content-Type": "application/json"})
print(json.load(urllib.request.urlopen(req, timeout=900))["choices"][0]["text"].strip())
```

Never the Chat tab or `/v1/chat/completions`.

### More

[USAGE.md](USAGE.md) also covers: a one-shot `llama-completion` run, a script that rewrites a whole folder (splitting long files, resampling over-copied pieces, listing numbers to check), long documents, Chinese specifics, a quality checklist and troubleshooting. To verify a setup, run the self-test in [AGENTS.md](AGENTS.md#8-self-test-verify-the-install).

## Before and after

First samples from the held-out evaluation set, not edited. Hand-picked and fact-checked by hand; error rates over the whole set are below.

<img src="assets/compare-en-email.png" alt="Work email: draft and rewrite" width="100%">

<img src="assets/compare-zh-email.png" alt="Chinese work email: draft and rewrite" width="100%">

More examples (a Reddit post, a Zhihu answer) are in the [GitHub README](https://github.com/sgaofen/humanize-model#before-and-after).

## Results

Evaluation set: 312 drafts (210 English, 102 Chinese), 18 genres, written from scratch by GLM-5.3, GPT-5.6 luna and Claude Sonnet (about a third each), never used in training. Two samples per draft.

**AI detection (external check only).** Originality.ai, API v3, AI Allowance 0% (strictest), 2026-10-01, 210 English drafts, first sample each.

| Model | Flagged as AI | Judged human |
|---|---|---|
| **humanizer 12B (this release)** | **26 / 210 (12%)** | **88%** |
| humanizer E4B (previous release, r7) | 26 / 210 (12%) | 88% |
| Early 12B checkpoint (R12s12b, temperature 0.85) | 61 / 210 (29%) | 71% |

Public baseline: the `blader/humanizer` skill (v3.1.0, 53k GitHub stars), applied by Claude Sonnet to the same 60 drafts: 60 / 60 flagged as AI (median AI score 100%). This release on those 60: 10 / 60 flagged. Flagged as AI by genre (all 210 drafts, same setting): social posts with emoji, hashtags or "1/ 2/" threads **8 / 16**, formal policy memos **5 / 13**, blog posts 3 / 16, essays 4 / 38, work reports 2 / 20, paper sections 2 / 22, Reddit posts 1 / 18, emails 1 / 35, forum answers 0 / 18, product reviews 0 / 14 (total 26 / 210). Failures concentrate in the most templated genres. Detectors change; this is one measurement on one date, not a promise.

<img src="assets/results-detector-en.png" alt="Originality.ai pass rates" width="100%">

**Fact fidelity.** English, 420 outputs, LLM judge GLM-5.3, one strict vote per output. Lower is better.

| | **12B (this release)** | E4B (previous, r7) |
|---|---|---|
| Severe fact error (changed a number, an event or the meaning) | **51 / 420 (12%)** | 68 / 409 (17%) |
| "Severe" on a three-level scale | **42 / 420 (10%)** | 71 / 420 |
| Dropped a format element | **35 / 420** | 53 / 409 |
| Median reuse (overlap with the draft) | **0.19** | 0.31 |
| Outputs with reuse > 0.5 | **1.0%** | 5.5% |

Chinese: the judge passed 135 of 204 outputs (66%). Severe errors are mostly a single number or word. The evaluation pipeline resamples once with an anti-copy penalty when an output copies more than 35% of the draft: 0 of 420 English and 17 of 204 Chinese outputs; the app never does this automatically (press *Regenerate*). **Proofread numbers, dates and names before you use the output.**

<img src="assets/results-fidelity-en.png" alt="Fact fidelity vs. the previous release" width="100%">

## Training

**No AI detector was used anywhere in training**: not as a reward, not as a filter, not to pick a checkpoint.

1. **SFT, 28,598 pairs** of AI draft → real human original. The human side is always real human writing (paper abstracts, government reports, student essays, company and mailing-list email, Reddit, Hacker News, Zhihu…); the AI side is a draft a frontier model wrote back from the human text.
2. **DPO, about 4,100 preference pairs**, chosen only on fact fidelity and copying (LLM judge GLM-5.3).
3. **GRPO**, 200 steps with a strict single-vote fact judge, then 150 steps of **RLRt** (16 drafts × 8 samples per step, temperature 1.0). Reward: an LLM judge reads the whole rewrite against the draft (severe errors, invented content, changed meaning and dropped formatting cost), plus a copy penalty on 5-gram and syntactic-skeleton reuse (free below .22, then linear).
4. Release = the final RLRt checkpoint.

<img src="assets/training-en.png" alt="Training pipeline" width="100%">

## Speed

Measured on an M5 Max. llama.cpp Q8_0 with Metal (what the app uses): about 36–38 tokens/s; a hundred-word email takes about 3.6 s, a Chinese email of about 300 characters about 8.5 s. MLX 8-bit: about 30 tokens/s in English and 38 tokens/s in Chinese; a hundred-word email takes about 9 s.

## Limitations

- Still makes fact errors: 12% of English outputs had a severe one on our set, usually one number or one word.
- Chinese is weaker than English (66% passed the judge).
- Templated genres (emoji/hashtag social posts, policy memos) are still often flagged by detectors.
- Formatting can change: 35 of 420 outputs dropped a format element; paragraph breaks, lists and headings sometimes merge or disappear.
- In casual genres it sometimes adds slang or profanity that wasn't in the draft.
- Detector results change over time. Nothing here guarantees any detector outcome.
- It is a writing tool for your own drafts. Where a school, employer or publication has rules about AI assistance, follow them.

## License

Apache License 2.0, for both the weights and the code. Fine-tuned from [google/gemma-4-12B](https://huggingface.co/google/gemma-4-12B), which Google releases under Apache 2.0. This project is not affiliated with or endorsed by Google.
