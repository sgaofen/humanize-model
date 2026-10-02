# Evaluation set and raw results

Everything behind the numbers in the main README. Nothing here was used in training.

**中文说明见下半部分。**

## Contents

| Path | What it is |
|---|---|
| `drafts/300a/`, `drafts/300b/` | The 312 drafts (210 English, 102 Chinese; files starting with `zh_` are Chinese). Each draft was written from scratch by a frontier model; the writer is the last part of the file name (`glm` = GLM-5.3, `luna` = GPT-5.6 luna, `sonnet` = Claude Sonnet). The genre is the first part. |
| `outputs/humanizer-12b_300{a,b}.json` | Outputs of the released 12B model: `{case: [{"text": ...}, {"text": ...}]}`, two samples per draft, temperature 1.0, top-p 0.95. |
| `outputs/baseline-blader-humanizer-skill-sonnet_300{a,b}.json` | Public baseline: Claude Sonnet following the `blader/humanizer` skill (v3.1.0), one rewrite each for the 60-draft detector subset. |
| `fidelity/humanizer-12b_en_300{a,b}.json`, `fidelity/humanizer-12b_zh_300{a,b}.json` | Fact-fidelity verdicts from the LLM judge (GLM-5.3, one vote, strict), one per output: facts kept, meaning changed, content added, greeting/sign-off kept, format kept, severity, evidence. |
| `originality/*.jsonl` | Originality.ai results (API v3, AI Allowance 0% = strictest setting), measured 2026-10-01 on the first sample of each English draft. `label: true` = judged AI. `ai` = the detector's AI score. `_s60` files cover the 60-draft subset only. |

## Detector results in these files

| File | Judged AI |
|---|---|
| `humanizer-12b.jsonl` (this release) | 26 / 210 |
| `humanizer-e4b-r7.jsonl` (previous release) | 26 / 210 |
| `early-12b-R12s12b-t085.jsonl` | 61 / 210 |
| `early-12b-sftv2-RLBh.jsonl` | 115 / 210 |
| `early-12b-R12s12b-t100_s60.jsonl` | 11 / 60 |
| `baseline-blader-humanizer-skill-sonnet_s60.jsonl` | 60 / 60 |

One measurement on one date. Detectors change.

---

# 评测集与原始结果

主 README 里所有数字的原始数据都在这里。这些数据没有用于训练。

- `drafts/`:312 篇草稿(英文 210、中文 102,`zh_` 开头的是中文),由三个前沿模型从零写成,文件名最后一段是写手(`glm` = GLM-5.3、`luna` = GPT-5.6 luna、`sonnet` = Claude Sonnet),第一段是体裁。
- `outputs/humanizer-12b_*`:发布版 12B 的输出,每篇 2 发,温度 1.0、top-p 0.95。
- `outputs/baseline-blader-humanizer-skill-sonnet_*`:公开基线(Claude Sonnet 按 `blader/humanizer` skill 改写),只含检测器那 60 篇小样本。
- `fidelity/`:LLM 判官(GLM-5.3,单票从严)对每一发的事实忠实度判定。
- `originality/`:Originality.ai(API v3,AI Allowance 0% 最严档)2026-10-01 实测,每篇英文草稿的第 1 发;`label: true` 表示被判为 AI。

检测结果只代表这一天、这一档位的一次测量,检测器会更新。
