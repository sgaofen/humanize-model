# AGENTS.md: install and call humanizer

Instructions for AI agents (Claude Code, Codex, Cursor, etc.) setting up **humanizer** for a user. Follow the steps in order. Every command can be pasted as is. Human-oriented docs: [README.md](https://github.com/sgaofen/humanize-model/blob/main/README.md), [USAGE.md: usage without the app](https://github.com/sgaofen/humanize-model/blob/main/docs/USAGE.md) (every runtime, batch script, long documents), [INSTALL.md](https://github.com/sgaofen/humanize-model/blob/main/docs/INSTALL.md). This file is also in the root of the Hugging Face repo, next to `USAGE.md`.

## 0. Facts you need

- **What it does:** rewrites one AI-written draft (English or Chinese) so it reads like a person wrote it. It is trained to keep every number, unit, date, name and quote, and to add nothing. Output = the rewritten text only.
- **Kind of model:** 12B **text-completion** model (fine-tuned from `google/gemma-4-12B`). **Not a chat model.** Never use a chat template, a system prompt, or `/v1/chat/completions`.
- **Weights:** Hugging Face repo `jialinyyzz/humanizer`. Recommended file: `humanizer-12b-Q8_0.gguf` (12,669,627,840 bytes, about 12.7 GB); `humanizer-12b-Q6_K.gguf` (10,029,797,088 bytes, about 10.0 GB) for 16 GB machines. You also need `prompt_format.json` from the same repo.
- **Runtime:** `llama-server` from llama.cpp (macOS, Windows, Linux). Alternatives in section 10.
- **Sampling:** temperature 1.0, top_p 0.95, and nothing else: top_k 0, min_p 0, repeat_penalty 1.0. Stop on EOS only. No stop strings.
- **License:** Apache 2.0.

## 1. Pick the route

- **The user wants an app, not code:** send them to <https://github.com/sgaofen/humanize-model/releases/latest> and have them download `Humanizer-<version>-macos-arm64.dmg` (Mac with Apple silicon) or `Humanizer-<version>-windows-x64-setup.exe` (Windows x64). The app is unsigned; the first-launch fix is in [INSTALL.md](https://github.com/sgaofen/humanize-model/blob/main/docs/INSTALL.md#first-launch-warnings). The app downloads the model itself. **You are done.**
- **Otherwise** (scripts, pipelines, a local API): continue with step 2.

## 2. Check the machine

```bash
# memory in GB
sysctl -n hw.memsize | awk '{print $1/1073741824}'                         # macOS
free -g | awk '/Mem:/{print $2}'                                           # Linux
powershell -c "(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory/1GB"   # Windows
```

| Memory | Use |
|---|---|
| 32 GB or more | `humanizer-12b-Q8_0.gguf` (12,669,627,840 bytes) |
| 16 GB | `humanizer-12b-Q6_K.gguf` (10,029,797,088 bytes) |
| 8 GB | `lite/humanizer-lite-Q6_K.gguf` (about 6.2 GB): the previous, smaller E4B release. Same prompt format. |

`humanizer-12b-Q4_K_M.gguf` is **not published yet** (it waits for the fact judge); don't try to download it. You also need free disk space for the file you pick. In the commands below, replace the Q8_0 file name if you picked another file.

## 3. Install llama.cpp

```bash
brew install llama.cpp          # macOS
winget install llama.cpp        # Windows
# Linux, or any OS: prebuilt zips at https://github.com/ggml-org/llama.cpp/releases
# (Windows + NVIDIA: the CUDA build; other GPUs: the Vulkan build), or build from source:
#   cmake -B build -DGGML_CUDA=ON && cmake --build build --config Release -j
llama-server --version          # must print a version
```

The app ships llama.cpp build `b11335`; use that build or a newer one.

## 4. Download the model

```bash
pip install -U "huggingface_hub[cli]"
hf download jialinyyzz/humanizer humanizer-12b-Q8_0.gguf prompt_format.json --local-dir ./humanizer-model
# Slow from mainland China? Prefix the command with HF_ENDPOINT=https://hf-mirror.com
```

Check the download:

```bash
head -c 4 ./humanizer-model/humanizer-12b-Q8_0.gguf; echo     # must print GGUF
wc -c ./humanizer-model/*.gguf                                # Q8_0: 12669627840 bytes; Q6_K: 10029797088 bytes
# sha256: [TBD: publish the sha256 of each GGUF and compare with shasum -a 256 / sha256sum]
```

## 5. Start the server

```bash
llama-server -m ./humanizer-model/humanizer-12b-Q8_0.gguf -c 8192 -np 1 -ngl 99 --host 127.0.0.1 --port 8080
```

Wait until it is ready (loading takes a while the first time):

```bash
curl -s http://127.0.0.1:8080/health        # ready when it returns {"status":"ok"}
```

`-np 1` gives the whole 8192-token context to one request (newer builds otherwise split it across parallel slots). `-ngl 99` puts all layers on the GPU (Metal, CUDA or Vulkan). If it runs out of GPU memory, lower `-ngl`. Check the log for `offloaded N/N layers`; if no layers are offloaded it runs on the CPU and will be slow.

## 6. Build the prompt, byte for byte

```python
import hashlib, json

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))
INSTR, SEP = PF["instr"], PF["sep"]          # SEP == "\n\n### Rewritten:\n\n"

def build_prompt(draft: str) -> str:
    return INSTR + "\n\n" + draft.strip() + SEP

# Must hold. If not, the prompt is wrong and the model will be worse.
assert hashlib.sha256(build_prompt("X").encode("utf-8")).hexdigest()[:16] == "cc51d66b4c593fbe"
```

If you cannot read `prompt_format.json`, `INSTR` is exactly this text (lines joined with `\n`, blank lines included, no trailing newline):

```
Rewrite the text below so it reads like a person wrote it, not a language model.

Reorganize it as you see fit. Vary sentence length on purpose. Cut hedging,
throat-clearing, and any sentence that only announces what comes next.
Prefer the concrete word over the abstract one. It is fine to sound uneven.

Every fact, number, unit, date, name and quotation must survive unchanged.
```

Do **not** reword it, translate it, add a system prompt, or wrap it in chat turns.

## 7. Call it

`POST http://127.0.0.1:8080/completion` with:

```json
{
  "prompt": "<build_prompt(draft)>",
  "temperature": 1.0,
  "top_p": 0.95,
  "top_k": 0,
  "min_p": 0,
  "repeat_penalty": 1.0,
  "n_predict": 2048
}
```

- Do not send `stop`. Generation ends at EOS.
- `top_k`, `min_p` and `repeat_penalty` must be set explicitly: llama-server's defaults (top_k 40, min_p 0.05) differ from how the model was evaluated.
- The rewrite is in the `content` field. Strip leading and trailing whitespace.
- `n_predict`: about 2.5× the draft's token count, between 256 and 2048 (the app's rule). 2048 is a safe fixed value.

A reusable function, standard library only:

```python
import json, urllib.request

def humanize(draft: str, url: str = "http://127.0.0.1:8080/completion") -> str:
    body = {"prompt": build_prompt(draft), "temperature": 1.0, "top_p": 0.95,
            "top_k": 0, "min_p": 0, "repeat_penalty": 1.0, "n_predict": 2048}
    req = urllib.request.Request(url, json.dumps(body).encode("utf-8"),
                                 {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=600) as r:
        return json.load(r)["content"].strip()
```

## 8. Self-test: verify the install

Save as `selftest.py` next to `humanizer-model/` and run `python3 selftest.py` while the server is up. Standard library only. The test draft is a real draft from the evaluation set.

```python
import hashlib, json, re, urllib.request

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))
build = lambda d: PF["instr"] + "\n\n" + d.strip() + PF["sep"]
assert hashlib.sha256(build("X").encode()).hexdigest()[:16] == "cc51d66b4c593fbe", "FAIL: prompt format"

DRAFT = """Hi Nadia,

I’m handing off the Meridian Bank loan audit to you. We’ve completed the review of 88 files, with 12 files remaining. The outstanding files are in the “Pending Review” folder in the Meridian Bank audit workspace, organized by borrower name.

The remaining items are primarily commercial real estate loans and include several files requiring confirmation of updated income statements, collateral valuations, and covenant documentation. I’ve flagged the missing items in the audit tracker and added notes for the files that may need follow-up with Meridian Bank’s loan operations team.

The deadline is Friday, March 14. Please prioritize the files marked “High” in the tracker, especially MB-2047, MB-2091, and MB-2110, since those are scheduled for management review first. I’ll be available through Wednesday afternoon to answer questions or provide context on any earlier decisions.

Thanks for taking this over."""

body = {"prompt": build(DRAFT), "temperature": 1.0, "top_p": 0.95, "top_k": 0, "min_p": 0,
        "repeat_penalty": 1.0, "n_predict": 2048}
req = urllib.request.Request("http://127.0.0.1:8080/completion", json.dumps(body).encode(),
                             {"Content-Type": "application/json"})
out = json.load(urllib.request.urlopen(req, timeout=600))["content"].strip()
print(out, "\n" + "-" * 60)

problems = []
if len(out.split()) < 60: problems.append("output too short")
if "### Rewritten" in out or out.startswith("Rewrite the text below"): problems.append("prompt echoed: wrong endpoint or chat template")
if re.search(r"<start_of_turn>|<end_of_turn>|<\|turn", out): problems.append("chat markers in output: a chat template is being applied")
grams = lambda t: {tuple(t.lower().split()[i:i + 5]) for i in range(len(t.split()) - 4)}
if len(grams(DRAFT) & grams(out)) / max(1, len(grams(out))) > 0.5: problems.append("output copies the draft")
missing = [n for n in re.findall(r"\d+", DRAFT) if n not in re.findall(r"\d+", out)]

print("FAIL: " + "; ".join(problems) if problems else "PASS: install looks right")
if missing: print("NOTE: numbers not found in this sample:", missing,
                  "(the model can slip; rerun once. If it repeats, check sampling settings)")
```

`PASS` means the server, the file and the prompt are wired correctly. A `NOTE` about a number is a sampling slip, not an install problem, unless it happens every time.

## 9. Rules when you use it for a user

- Send **one draft per request**. Our setup uses an 8192-token context, so the draft plus the rewrite must fit. Split long documents at paragraph boundaries and rewrite the pieces separately ([USAGE.md section 9](https://github.com/sgaofen/humanize-model/blob/main/docs/USAGE.md#9-rewrite-a-whole-folder) has a ready-made batch script that does this).
- **Never edit the output silently**, and never tell the user the result is guaranteed to pass an AI detector. Detection numbers in the README are one measurement on one date.
- **Ask the user to proofread** numbers, dates, names and the direction of each claim. On the evaluation set, 51 of 420 English outputs (12%) had a severe fact error, usually a single number or word. Compare the numbers in the draft and the output yourself and point out any that differ.
- If an output copies most of the draft, or a number differs, **sample again** (same prompt; sampling is random).
- Chinese is weaker than English (135 of 204 Chinese outputs passed the fact judge).
- Formatting can change: paragraph breaks, lists and headings are sometimes dropped or merged.

## 10. Other runtimes

All of them need the same prompt (step 6) and the same sampling (step 7). Complete code for each is in [USAGE.md](https://github.com/sgaofen/humanize-model/blob/main/docs/USAGE.md).

- **MLX (Apple silicon):** `mlx_lm.convert --hf-path jialinyyzz/humanizer --mlx-path humanizer-mlx-8bit -q --q-bits 8 --q-group-size 64`, then `mlx_lm.generate`'s Python API with `make_sampler(temp=1.0, top_p=0.95)`. Pass the prompt string directly; don't apply a chat template.
- **transformers (CUDA):** `AutoModelForCausalLM.from_pretrained("jialinyyzz/humanizer", dtype=torch.bfloat16)`, `generate(do_sample=True, temperature=1.0, top_p=0.95, top_k=0)`. `top_k=0` overrides the top-k 64 in the bundled `generation_config.json`. The bf16 `model.safetensors` (about 24 GB) is at the repo root.
- **vLLM:** `LLM(model="jialinyyzz/humanizer", dtype="bfloat16", max_model_len=8192, limit_mm_per_prompt={"image": 0, "audio": 0, "video": 0})` and `SamplingParams(temperature=1.0, top_p=0.95, top_k=-1, min_p=0.0, repetition_penalty=1.0, max_tokens=2048)`. For `vllm serve`, add `--generation-config vllm` and use `/v1/completions`.
- **Ollama:** Modelfile with `TEMPLATE """{{ .Prompt }}"""`, then `/api/generate` with `"raw": true`. Untested by the authors.
- **LM Studio:** text-completion endpoint `/v1/completions` with the full prompt. Never the chat endpoint. Untested by the authors.

## 11. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Output starts with "Sure", "Here is", or repeats the instruction | A chat template or chat endpoint is in use | Use `/completion` (or raw mode) and send the prompt from step 6 |
| Output is cut off at `###` | A stop string was set | Remove all stop strings; rely on EOS |
| Output is almost the same as the draft | Sampling bad luck, or temperature too low | Check temperature 1.0 and sample again |
| Rambling or repeated phrases | Wrong sampler settings | Set `top_k: 0, min_p: 0, repeat_penalty: 1.0` explicitly |
| Out of memory while loading | Not enough RAM or VRAM for Q8_0 | Use Q6_K (16 GB) or the lite model (8 GB); lower `-ngl` |
| Very slow | Running on the CPU | Check `offloaded N/N layers` in the log; install the Metal, CUDA or Vulkan build |
| 404 when downloading a file | Wrong name, or Q4_K_M (not published yet) | Use the file names in step 2 |
