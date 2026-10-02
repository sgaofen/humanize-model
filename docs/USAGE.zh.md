# 不用 App 怎么用 humanizer

[English](USAGE.md) · [README](https://github.com/sgaofen/humanize-model/blob/main/README.zh.md) · [AGENTS.md（给 AI Agent）](https://github.com/sgaofen/humanize-model/blob/main/AGENTS.md) · [Hugging Face 上的模型文件](https://huggingface.co/jialinyyzz/humanizer/tree/main)

这份文档写给不想用[桌面 App](https://github.com/sgaofen/humanize-model/releases/latest)、要在自己的代码或命令行里跑 humanizer 的人。每个代码块都可以直接复制。

**目录：**[1. 这个模型哪里特殊](#1-这个模型哪里特殊) · [2. 选哪个文件](#2-选哪个文件) · [3. llama.cpp](#3-llamacpp推荐) · [4. MLX](#4-mlxapple-芯片) · [5. transformers](#5-transformerscuda) · [6. vLLM](#6-vllm) · [7. Ollama](#7-ollama) · [8. LM Studio](#8-lm-studio) · [9. 批量改写一个文件夹](#9-批量改写一个文件夹) · [10. 长文](#10-长文) · [11. 中文](#11-中文) · [12. 质量检查清单](#12-质量检查清单) · [13. 排错](#13-排错)

## 1. 这个模型哪里特殊

humanizer 是一个 12B 的**文本续写**模型，由 `google/gemma-4-12B` 微调而来：给它一篇 AI 写的草稿，它接着写出改写。下面四条规则对所有运行方式都成立。做对了，效果和我们的评测一致；错一条，效果就会明显变差。

| 规则 | 原因 |
|---|---|
| **它不是聊天模型。**只发一个纯文本字符串：不要聊天模板、不要系统提示词、不要轮次标记、不要用 `/v1/chat/completions`。 | 它训练时见到的就是下面这种原始文本。聊天模板会给草稿套上它训练时从没见过的标记。 |
| **提示词必须逐字一致。** | 指令改写过、翻译过，或者少一个空行，效果都会变差。 |
| **只靠 EOS 停。**不要设停止符，尤其不要用 `###`。 | 模型会自己结束。少数正常输出里本来就有 `###`，会被截断。 |
| **采样：temperature 1.0、top-p 0.95，别的都关掉。**top-k 关（0），min-p 关（0），重复惩罚 1.0。 | 评测就是这样跑的。好几个运行环境默认会开别的采样：llama.cpp 默认 top-k 40、min-p 0.05；transformers 会从随权重附带的 `generation_config.json` 里读到 top-k 64。 |

### 提示词

```
prompt = INSTR + "\n\n" + draft.strip() + "\n\n### Rewritten:\n\n"
```

`INSTR` 就是下面这段原文（各行用 `\n` 连接，空行也算，结尾不带换行）：

```
Rewrite the text below so it reads like a person wrote it, not a language model.

Reorganize it as you see fit. Vary sentence length on purpose. Cut hedging,
throat-clearing, and any sentence that only announces what comes next.
Prefer the concrete word over the abstract one. It is fine to sound uneven.

Every fact, number, unit, date, name and quotation must survive unchanged.
```

- `draft.strip()` 是去掉草稿首尾的空白。
- 分隔符 `\n\n### Rewritten:\n\n` 结尾是一个空行，模型从这之后开始写。
- 中文草稿也用这段英文指令，**不要翻译**。
- 这两段字符串随权重放在 `prompt_format.json` 里（字段 `instr` 和 `sep`）。

一个可以贴到任何地方的提示词拼接函数，带自检：

```python
import hashlib

INSTR = (
    "Rewrite the text below so it reads like a person wrote it, not a language model.\n"
    "\n"
    "Reorganize it as you see fit. Vary sentence length on purpose. Cut hedging,\n"
    "throat-clearing, and any sentence that only announces what comes next.\n"
    "Prefer the concrete word over the abstract one. It is fine to sound uneven.\n"
    "\n"
    "Every fact, number, unit, date, name and quotation must survive unchanged."
)
SEP = "\n\n### Rewritten:\n\n"

def build_prompt(draft: str) -> str:
    return INSTR + "\n\n" + draft.strip() + SEP

# 指纹:这一行报错就说明提示词拼错了
assert hashlib.sha256(build_prompt("X").encode("utf-8")).hexdigest()[:16] == "cc51d66b4c593fbe"
```

**输出长度。**改写通常和草稿差不多长。输出上限留草稿 token 数的 2.5 倍左右；App 用的是 `min(2048, max(256, 2.5 × 草稿 token 数))`。指令、草稿和改写加起来共用 8192 token 的上下文，见[长文](#10-长文)。

## 2. 选哪个文件

文件都在 [`jialinyyzz/humanizer`](https://huggingface.co/jialinyyzz/humanizer/tree/main)：

| 内存 | 文件 | 大小 |
|---|---|---|
| 32 GB 及以上 | `humanizer-12b-Q8_0.gguf` | 12,669,627,840 字节（约 12.7 GB） |
| 16 GB | `humanizer-12b-Q6_K.gguf` | 10,029,797,088 字节（约 10.0 GB） |
| 8 GB | `lite/humanizer-lite-Q6_K.gguf`，即上一版更小的 E4B | 约 6.2 GB |
| （暂未发布） | `humanizer-12b-Q4_K_M.gguf` | 约 7.6 GB，*即将推出*：过了事实判官才发 |

仓库里还有：

- 根目录的 `model.safetensors`（bf16，约 24 GB），以及 `config.json`、`generation_config.json`、`tokenizer.json`、`tokenizer_config.json`、`prompt_format.json`：transformers、vLLM 和 MLX 转换都用它们。
- `lite/`（上一版 E4B）：`humanizer-lite-Q8_0.gguf`（约 8.0 GB）、`humanizer-lite-Q6_K.gguf`（约 6.2 GB）、`humanizer-lite-bf16.gguf`（约 14.9 GB），以及 4 个 safetensors 分片（约 15.9 GB）和 config、tokenizer、`prompt_format.json`。提示词格式一样。

**量化版和 bf16 差多少。**我们拿评测集里 104 篇草稿和它们的改写当测试文本（和校准数据不重叠）实测。Q6_K 和 Q4_K_M 用我们自己的改写数据做 imatrix 校准，词表和输出层保留 8 bit。

| 文件 | 与 bf16 的平均 KL | 首选词与 bf16 一致 | 困惑度 |
|---|---|---|---|
| Q8_0 | 0.0017 | 98.4% | +0.2% |
| Q6_K | 0.0033 | 97.8% | +0.5% |
| Q4_K_M | 0.0214 | 93.9% | +2.3% |

Q4_K_M 的损失明显更大，所以要等事实判官通过才发。

**下载：**

```bash
pip install -U "huggingface_hub[cli]"
hf download jialinyyzz/humanizer humanizer-12b-Q8_0.gguf prompt_format.json --local-dir ./humanizer-model
# 16 GB 的机器:把 humanizer-12b-Q8_0.gguf 换成 humanizer-12b-Q6_K.gguf
# 国内下载慢:在命令前面加 HF_ENDPOINT=https://hf-mirror.com
wc -c ./humanizer-model/*.gguf     # Q8_0 应为 12669627840 字节,Q6_K 应为 10029797088 字节
```

<!-- TBD: 上传后填每个 GGUF 的 sha256 -->
sha256 校验值：*即将推出*。

## 3. llama.cpp（推荐）

macOS（Metal）、Windows、Linux（CUDA、Vulkan 或 CPU）都能用。App 自己用的是 llama.cpp `b11335`，这个版本或更新的都可以。

### 安装

| 系统 | 命令 |
|---|---|
| macOS | `brew install llama.cpp` |
| Windows | `winget install llama.cpp`，或到 [llama.cpp Releases](https://github.com/ggml-org/llama.cpp/releases) 下 zip（NVIDIA 选 CUDA 版，其他显卡选 Vulkan 版） |
| Linux | 到 [llama.cpp Releases](https://github.com/ggml-org/llama.cpp/releases) 下 zip，或自己编译：`cmake -B build -DGGML_CUDA=ON && cmake --build build --config Release -j`（没有 NVIDIA 显卡就去掉 `-DGGML_CUDA=ON`） |

### 起服务

```bash
llama-server -m ./humanizer-model/humanizer-12b-Q8_0.gguf -c 8192 -np 1 -ngl 99 --host 127.0.0.1 --port 8080
```

- `-c 8192`：指令 + 草稿 + 改写共用的上下文。
- `-np 1`：一次只处理一个请求，整个上下文都给它（新版本默认会把上下文分给几个并行槽位）。
- `-ngl 99`：所有层放到显卡上。显存不够就调低。日志里 `offloaded N/N layers` 那一行说明它跑在哪。
- 本地还没有文件时，把 `-m …` 换成 `--hf-repo jialinyyzz/humanizer --hf-file humanizer-12b-Q8_0.gguf`，它会自己下载。

`curl -s http://127.0.0.1:8080/health` 返回 `{"status":"ok"}` 就是准备好了。

用 `/completion` 接口。不要用 `/v1/chat/completions`，它会套聊天模板。

### 用 Python 调用（只用标准库）

```python
import json, urllib.request

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))

def humanize(draft: str, url: str = "http://127.0.0.1:8080") -> str:
    body = {
        "prompt": PF["instr"] + "\n\n" + draft.strip() + PF["sep"],
        "temperature": 1.0, "top_p": 0.95, "top_k": 0, "min_p": 0, "repeat_penalty": 1.0,
        "n_predict": 2048,                       # 不传 "stop":模型在 EOS 处自己结束
    }
    req = urllib.request.Request(url + "/completion", json.dumps(body).encode("utf-8"),
                                 {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=900) as r:
        return json.load(r)["content"].strip()

print(humanize(open("draft.txt", encoding="utf-8").read()))
```

要流式输出就加 `"stream": true`，读服务端推送的事件，每个事件的 `content` 是接下来的一段文字。

### 用 curl 调用（macOS 和 Linux，需要 jq）

```bash
jq -n --rawfile d draft.txt --slurpfile f humanizer-model/prompt_format.json \
  '{prompt: ($f[0].instr + "\n\n" + ($d | sub("^\\s+"; "") | sub("\\s+$"; "")) + $f[0].sep),
    temperature: 1.0, top_p: 0.95, top_k: 0, min_p: 0, repeat_penalty: 1.0, n_predict: 2048}' \
| curl -s http://127.0.0.1:8080/completion -d @- | jq -r .content
```

### 不起服务，一次性跑

新版 llama.cpp 里，纯文本续写工具叫 `llama-completion`；老版本用 `llama-cli`，参数一样。`-no-cnv` 让它不进入聊天模式。

```bash
# 1) 把逐字的提示词写进文件。结尾多一个 "\n" 是故意的:
#    llama.cpp 用 -f 读文件时会去掉恰好一个结尾换行。
python3 - <<'EOF'
import json
pf = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))
draft = open("draft.txt", encoding="utf-8").read()
with open("prompt.txt", "w", encoding="utf-8", newline="") as f:
    f.write(pf["instr"] + "\n\n" + draft.strip() + pf["sep"] + "\n")
EOF

# 2) 跑一次,只打印改写
llama-completion -m ./humanizer-model/humanizer-12b-Q8_0.gguf -f prompt.txt -c 8192 -n 2048 -ngl 99 \
  -no-cnv --no-display-prompt --temp 1.0 --top-p 0.95 --top-k 0 --min-p 0 --repeat-penalty 1.0
```

我们自己用的是 llama-server，这条一次性跑的路没有测过。第一次跑时加上 `--verbose-prompt` 看看切好的提示词：结尾必须是 `Rewritten`、`:` 和一个空行，后面不能再有东西。

## 4. MLX（Apple 芯片）

需要 mlx-lm 0.32 或更新。12B 没有现成的 MLX 文件，要先把 bf16 权重转成 8 bit（只需转一次）。转换要读 24 GB 的 bf16 下载，32 GB 及以上内存的 Mac 比较从容。我们只实测过 8 bit 的转换：M5 Max 上英文约 30 token/s，中文约 38 token/s。

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
sampler = make_sampler(temp=1.0, top_p=0.95)          # top-k、min-p 保持默认的关闭状态

def humanize(draft: str) -> str:
    prompt = PF["instr"] + "\n\n" + draft.strip() + PF["sep"]
    return generate(model, tok, prompt=prompt, max_tokens=2048, sampler=sampler).strip()

print(humanize(open("draft.txt", encoding="utf-8").read()))
```

给 `generate()` 传纯字符串，不要调用 `tok.apply_chat_template`。命令行工具 `mlx_lm.generate` 默认会套聊天模板，要加 `--ignore-chat-template`；上面的 Python 接口不会套。lite（E4B）模型在 Mac 上请用它的 GGUF 配 llama.cpp。

## 5. transformers（CUDA）

bf16 权重约 24 GB，显存要比这更大，或者用 `device_map="auto"` 把模型分到几张卡上。transformers 要用支持 Gemma 4 的版本，权重是用 5.14.1 存的。

```bash
pip install -U torch transformers accelerate huggingface_hub
```

```python
import json, torch
from huggingface_hub import hf_hub_download
from transformers import AutoModelForCausalLM, AutoTokenizer

repo = "jialinyyzz/humanizer"
PF = json.load(open(hf_hub_download(repo, "prompt_format.json"), encoding="utf-8"))
tok = AutoTokenizer.from_pretrained(repo)
model = AutoModelForCausalLM.from_pretrained(repo, dtype=torch.bfloat16, device_map="auto")

def humanize(draft: str) -> str:
    ids = tok(PF["instr"] + "\n\n" + draft.strip() + PF["sep"], return_tensors="pt").to(model.device)
    out = model.generate(**ids, do_sample=True, temperature=1.0, top_p=0.95,
                         top_k=0,                # 关掉 generation_config.json 里的 top-k 64
                         max_new_tokens=2048)
    return tok.decode(out[0, ids["input_ids"].shape[1]:], skip_special_tokens=True).strip()

print(humanize(open("draft.txt", encoding="utf-8").read()))
```

transformers 4.x 要把 `dtype=` 写成 `torch_dtype=`。`top_k=0` 不能省，省了就是在 top-k 64 下采样。

## 6. vLLM

下面的离线接口和我们生成评测输出的方式一致（vLLM、bf16、temperature 1.0、top-p 0.95）；评测另外还用了防照抄重采。

```python
import json
from huggingface_hub import hf_hub_download
from vllm import LLM, SamplingParams

repo = "jialinyyzz/humanizer"
PF = json.load(open(hf_hub_download(repo, "prompt_format.json"), encoding="utf-8"))
llm = LLM(model=repo, dtype="bfloat16", max_model_len=8192,
          limit_mm_per_prompt={"image": 0, "audio": 0, "video": 0})   # 只用文本
params = SamplingParams(temperature=1.0, top_p=0.95, top_k=-1, min_p=0.0,
                        repetition_penalty=1.0, max_tokens=2048)      # top_k=-1 表示关闭

drafts = [open(p, encoding="utf-8").read() for p in ["draft1.txt", "draft2.txt"]]
prompts = [PF["instr"] + "\n\n" + d.strip() + PF["sep"] for d in drafts]
for result in llm.generate(prompts, params):
    print(result.outputs[0].text.strip(), "\n---")
```

起成服务（兼容 OpenAI 接口）。`--generation-config vllm` 让 vLLM 不把 `generation_config.json` 里的 top-k 64 当默认值：

```bash
vllm serve jialinyyzz/humanizer --dtype bfloat16 --max-model-len 8192 --generation-config vllm
```

```bash
jq -n --rawfile d draft.txt --slurpfile f humanizer-model/prompt_format.json \
  '{model: "jialinyyzz/humanizer",
    prompt: ($f[0].instr + "\n\n" + ($d | sub("^\\s+"; "") | sub("\\s+$"; "")) + $f[0].sep),
    temperature: 1.0, top_p: 0.95, top_k: -1, min_p: 0, repetition_penalty: 1.0, max_tokens: 2048}' \
| curl -s http://127.0.0.1:8000/v1/completions -H "Content-Type: application/json" -d @- \
| jq -r '.choices[0].text'
```

用 `/v1/completions`，不要用 `/v1/chat/completions`。服务这条路我们自己没测过。

## 7. Ollama

不做设置的话，Ollama 会套聊天模板。要建一个把提示词原样透传的模型，再用原始（raw）模式调接口。Ollama 的引擎要支持 Gemma 4 才能加载。Ollama 我们自己没测过。

`Modelfile`（和 GGUF 放在一起）：

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
ollama show humanizer --modelfile      # 确认没有被自动加上 "PARAMETER stop" 之类的行
```

调 `/api/generate`，带 `"raw": true` 和完整提示词（指令 + 草稿 + 分隔符）：

```bash
jq -n --rawfile d draft.txt --slurpfile f humanizer-model/prompt_format.json \
  '{model: "humanizer", raw: true, stream: false,
    prompt: ($f[0].instr + "\n\n" + ($d | sub("^\\s+"; "") | sub("\\s+$"; "")) + $f[0].sep),
    options: {temperature: 1.0, top_p: 0.95, top_k: 0, min_p: 0, repeat_penalty: 1.0,
              num_ctx: 8192, num_predict: 2048}}' \
| curl -s http://127.0.0.1:11434/api/generate -d @- | jq -r .response
```

Python：

```python
import json, urllib.request

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))
draft = open("draft.txt", encoding="utf-8").read()
body = {"model": "humanizer", "raw": True, "stream": False,
        "prompt": PF["instr"] + "\n\n" + draft.strip() + PF["sep"],
        "options": {"temperature": 1.0, "top_p": 0.95, "top_k": 0, "min_p": 0, "repeat_penalty": 1.0,
                    "num_ctx": 8192, "num_predict": 2048}}
req = urllib.request.Request("http://127.0.0.1:11434/api/generate", json.dumps(body).encode("utf-8"),
                             {"Content-Type": "application/json"})
print(json.load(urllib.request.urlopen(req, timeout=900))["response"].strip())
```

不要用交互式的 `ollama run` 或 `/api/chat`，它们都会走聊天格式。

## 8. LM Studio

LM Studio 我们自己没测过。

1. 加载 `humanizer-12b-Q8_0.gguf`（或 Q6_K），加载时把上下文长度设成 8192。
2. 在模型的采样设置里设 **Temperature 1.0、Top P 0.95、Top K 0、Min P 0、Repeat Penalty 1.0**，删掉所有停止符。
3. 在 Developer 页开本地服务，把完整提示词（指令 + 草稿 + 分隔符）发到**文本续写接口 `/v1/completions`**：

```python
import json, urllib.request

PF = json.load(open("humanizer-model/prompt_format.json", encoding="utf-8"))
draft = open("draft.txt", encoding="utf-8").read()
body = {"model": "humanizer-12b-q8_0",         # 换成 LM Studio 里显示的模型 id
        "prompt": PF["instr"] + "\n\n" + draft.strip() + PF["sep"],
        "temperature": 1.0, "top_p": 0.95, "top_k": 0, "min_p": 0, "repeat_penalty": 1.0,
        "max_tokens": 2048}
req = urllib.request.Request("http://127.0.0.1:1234/v1/completions", json.dumps(body).encode("utf-8"),
                             {"Content-Type": "application/json"})
print(json.load(urllib.request.urlopen(req, timeout=900))["choices"][0]["text"].strip())
```

不要用聊天页面，也不要用 `/v1/chat/completions`，两者都会套聊天模板。如果你的 LM Studio 版本不认请求里的 `top_k`、`min_p`、`repeat_penalty`，第 2 步的设置会生效。

## 9. 批量改写一个文件夹

`humanize_folder.py` 通过正在运行的 llama-server（见[第 3 节](#起服务)），把一个文件夹里的每个 `.txt` 改写到另一个文件夹。只用标准库。

- 长文件按空行切成若干段，每段的草稿不超过 `--max-tokens` 个 token；逐段改写后用空行拼起来。
- 某段改写照抄草稿超过 `--max-copy`（默认 0.35）时，这段会再采一次，保留照抄更少的那一版。
- 草稿里有、改写里找不到的数字写进 `report.tsv`，供你人工核对。
- 输出文件夹里已经有的文件会跳过，可以中途停下再接着跑。

```bash
python3 humanize_folder.py drafts/ rewrites/
python3 humanize_folder.py drafts/ rewrites/ --url http://127.0.0.1:8080 --max-tokens 1500
```

```python
#!/usr/bin/env python3
"""Rewrite every .txt file in a folder with humanizer, through a running llama-server.

    python3 humanize_folder.py drafts/ rewrites/
    python3 humanize_folder.py drafts/ rewrites/ --url http://127.0.0.1:8080 --max-tokens 1500

Standard library only. Long files are split at blank lines into pieces of at most --max-tokens
draft tokens; each piece is rewritten on its own and the pieces are joined with a blank line.
A piece whose rewrite copies more than --max-copy of the draft is sampled once more and the
less-copied version is kept. Numbers that appear in a draft but not in its rewrite are listed in
report.tsv so you can check them by hand. Files already in the output folder are skipped.
"""
import argparse, hashlib, json, math, os, re, sys, urllib.request

INSTR = (
    "Rewrite the text below so it reads like a person wrote it, not a language model.\n"
    "\n"
    "Reorganize it as you see fit. Vary sentence length on purpose. Cut hedging,\n"
    "throat-clearing, and any sentence that only announces what comes next.\n"
    "Prefer the concrete word over the abstract one. It is fine to sound uneven.\n"
    "\n"
    "Every fact, number, unit, date, name and quotation must survive unchanged."
)
SEP = "\n\n### Rewritten:\n\n"


def build_prompt(draft):
    return INSTR + "\n\n" + draft.strip() + SEP


assert hashlib.sha256(build_prompt("X").encode("utf-8")).hexdigest()[:16] == "cc51d66b4c593fbe"


def post(url, path, body, timeout=1800):
    req = urllib.request.Request(url.rstrip("/") + path, json.dumps(body).encode("utf-8"),
                                 {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)


def n_tokens(url, text):
    return len(post(url, "/tokenize", {"content": text})["tokens"])


def split_long(url, text, max_tokens):
    """Group paragraphs (split at blank lines) into pieces of at most max_tokens tokens."""
    paras = [p.strip() for p in re.split(r"\n\s*\n", text.strip()) if p.strip()]
    pieces, cur = [], []
    for p in paras:
        if cur and n_tokens(url, "\n\n".join(cur + [p])) > max_tokens:
            pieces.append("\n\n".join(cur))
            cur = []
        cur.append(p)
    if cur:
        pieces.append("\n\n".join(cur))
    return pieces


def rewrite(url, draft, ctx):
    prompt = build_prompt(draft)
    room = ctx - n_tokens(url, prompt) - 16
    n_predict = max(256, min(math.ceil(n_tokens(url, draft) * 2.5), room))
    body = {"prompt": prompt, "n_predict": n_predict, "temperature": 1.0, "top_p": 0.95,
            "top_k": 0, "min_p": 0, "repeat_penalty": 1.0}            # no "stop": ends at EOS
    return post(url, "/completion", body)["content"].strip()


# Rough copy ratio: share of the rewrite's 5-grams (words; single CJK characters) found in the draft.
UNIT = re.compile(r"[㐀-鿿]|[^\W_㐀-鿿]+|[^\w\s]")


def copy_ratio(draft, out, n=5):
    a = [t.lower() for t in UNIT.findall(draft)]
    b = [t.lower() for t in UNIT.findall(out)]
    seen = {tuple(a[i:i + n]) for i in range(len(a) - n + 1)}
    grams = [tuple(b[i:i + n]) for i in range(len(b) - n + 1)]
    return sum(g in seen for g in grams) / len(grams) if grams else 0.0


NUM = re.compile(r"\d+(?:[.,:/]\d+)*")


def missing_numbers(draft, out):
    have = {m.replace(",", "") for m in NUM.findall(out)} | set(re.findall(r"\d+", out))
    return sorted({m.replace(",", "") for m in NUM.findall(draft)} - have)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("src"); ap.add_argument("dst")
    ap.add_argument("--url", default="http://127.0.0.1:8080")
    ap.add_argument("--ctx", type=int, default=8192, help="the -c you gave llama-server")
    ap.add_argument("--max-tokens", type=int, default=1500, help="max draft tokens per piece")
    ap.add_argument("--max-copy", type=float, default=0.35, help="resample a piece once above this")
    a = ap.parse_args()
    os.makedirs(a.dst, exist_ok=True)
    files = sorted(f for f in os.listdir(a.src) if f.lower().endswith(".txt"))
    report = open(os.path.join(a.dst, "report.tsv"), "a", encoding="utf-8")
    for i, name in enumerate(files, 1):
        out_path = os.path.join(a.dst, name)
        if os.path.exists(out_path):
            print(f"[{i}/{len(files)}] {name}: already done, skipped"); continue
        draft = open(os.path.join(a.src, name), encoding="utf-8").read()
        if not draft.strip():
            continue
        parts = []
        for piece in split_long(a.url, draft, a.max_tokens):
            out = rewrite(a.url, piece, a.ctx)
            c = copy_ratio(piece, out)
            if c > a.max_copy:
                out2 = rewrite(a.url, piece, a.ctx)
                c2 = copy_ratio(piece, out2)
                if c2 < c:
                    out, c = out2, c2
            parts.append(out)
        text = "\n\n".join(parts)
        with open(out_path, "w", encoding="utf-8", newline="") as f:
            f.write(text + "\n")
        miss = missing_numbers(draft, text)
        report.write(f"{name}\t{len(parts)} piece(s)\tcopy {copy_ratio(draft, text):.2f}\tcheck numbers: {' '.join(miss) or '-'}\n")
        report.flush()
        print(f"[{i}/{len(files)}] {name}: {len(parts)} piece(s), copy {copy_ratio(draft, text):.2f}"
              + (f", check numbers {miss}" if miss else ""))


if __name__ == "__main__":
    main()
```

这里的照抄率是粗略估计（改写里连续 5 个词或 5 个字的片段有多少出现在草稿里），不是我们评测用的精确指标；第二次采样也不带防照抄惩罚，和评测里的重采不同。

## 10. 长文

- 指令、草稿和改写加起来共用 **8192 token** 的上下文。改写和草稿差不多长，所以每段草稿要明显少于一半。
- 按**段落**（空行）切开，不要在句子中间切，然后分段改写。上面的批处理脚本就是这么做的，默认每段最多 1,500 token。
- 模型训练和评测用的都是单篇邮件、帖子、作文、报告段落，几百词的长度，这个长度的段效果最好。
- 每段改写时看不到其他段，所以段与段之间语气可能略有变化，事实也不会在段之间挪动。拼好之后从头到尾读一遍。
- 标题、代码块和列表常被删掉或改成正文。只改写正文，标题和代码自己留着；[`humanizer/markdown_guard.py`](https://github.com/sgaofen/humanize-model/blob/main/humanizer/markdown_guard.py) 就是这样逐块处理的。

## 11. 中文

- 用**同一段英文指令**，不要翻译。
- 中文**弱于英文**：我们的事实判官在 204 发中文输出里判通过 135 发（66%）。
- 中文改写更容易照抄草稿。评测里，防照抄重采（照抄超过 35% 时触发）在中文 204 发里触发了 17 发，英文 420 发里是 0 发。改写看着和草稿太像就再采一次；批处理脚本会自动这样做。
- 中文里**数字更常换写法**：汉字数字变成阿拉伯数字（三 → 3），日期改格式（6月14日 → 6.14）。App 和批处理脚本里那种只看阿拉伯数字的核对发现不了这类问题，请读一遍核对。
- 全角、半角标点可能互换；问候、正文、落款有时会被并成一段。发出去之前把格式理一理。
- 速度和英文差不多：llama.cpp Q8_0 在 M5 Max 上，一封 300 字左右的中文邮件约 8.5 秒。

## 12. 质量检查清单

- **通读一遍改写。**核对每个数字、日期、单位和人名，以及每个论断的方向（谁做了什么、多还是少、先还是后）。在我们的评测集上，英文 420 发里有 51 发（12%）出现严重事实错，多数只错一个数字或一个词。
- **补回需要的格式：**主题行、列表、标题、落款有时会丢（420 发里有 35 发）。
- **和草稿太像？再采一次。**每次都是重新采样。
- **随意体裁里语气会跑。**Reddit 一类的帖子里，它有时会加上原文没有的俚语或粗口，请删掉。
- **不保证过检测器。**我们的检测数字是一个检测器在某一天的一次测量；模板化的体裁（带 emoji、井号的社交帖，政策备忘）仍然常被判 AI。这里不对任何检测结果做承诺。
- 这是给你改自己草稿的写作工具。学校、单位或出版方对 AI 辅助有规定的，请按规定来。

## 13. 排错

| 现象 | 原因 | 解决 |
|---|---|---|
| 输出以“好的”“Sure”“Here is…”开头，或者复述指令 | 用了聊天模板或聊天接口 | 用续写接口（`/completion`、`/v1/completions`、Ollama 的 `raw: true`），发[第 1 节](#1-这个模型哪里特殊)里逐字的提示词 |
| 输出里有 `<start_of_turn>`、`<end_of_turn>` 之类的标记 | 同上：聊天格式 | 同上 |
| 输出在 `###` 处或很早就停了 | 设了停止符 | 删掉所有停止符，只靠 EOS |
| 改写和草稿几乎一样 | 采样运气不好，或温度太低 | 确认 temperature 1.0，再采一次 |
| 胡言乱语、用词古怪或反复重复 | 采样参数不对（llama.cpp 默认的 top-k 40 / min-p 0.05、`generation_config.json` 里的 top-k 64，或者开了重复惩罚） | 显式设 top-k 0、min-p 0、重复惩罚 1.0 |
| 改写在句子中间断了 | 输出上限或上下文太小 | 调大 `n_predict` / `max_tokens`；llama-server 加 `-c 8192 -np 1`；长稿分段 |
| 加载时内存不够 | 文件对你的内存或显存太大 | 16 GB 用 Q6_K，8 GB 用 lite；调低 `-ngl` |
| 很慢 | 在用 CPU 跑 | 看 llama.cpp 日志里有没有 `offloaded N/N layers`；装 Metal、CUDA 或 Vulkan 版 |
| 下载时 404 | 文件名写错，或文件还没发布（Q4_K_M） | 用[第 2 节](#2-选哪个文件)里的文件名 |
| 指纹自检不过 | 提示词拼法和训练时不一样 | 直接复制[第 1 节](#提示词)里的 `build_prompt` |

**自检。**[AGENTS.md 第 8 节](https://github.com/sgaofen/humanize-model/blob/main/AGENTS.md#8-self-test-verify-the-install)有一个只用标准库的脚本：它把评测集里的一篇真实草稿发给 llama-server，检查提示词格式、接口、聊天模板有没有漏进来、有没有照抄。设置正确会打印 `PASS`。
