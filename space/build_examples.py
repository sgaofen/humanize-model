#!/usr/bin/env python3
"""Build examples.json for the Space from the public evaluation set in ../eval.

    python3 space/build_examples.py

Each example is a held-out draft and the FIRST sample for it from the Q8_0 file you download
(humanizer-12b-Q8_0.gguf, plain llama.cpp sampling with the app's settings), unedited:
eval/outputs/humanizer-12b-Q8_0_300{a,b}.json, index 0. Only whitespace is normalised for display.
The fact-judge verdict for that same sample comes from eval/fidelity/humanizer-12b-Q8_0_{en,zh}_300{a,b}.json;
a pick must be clean on that first pass (English: severity "none", facts kept, nothing added, meaning
unchanged; Chinese: facts kept, nothing added) and must not appear in the second-pass fix list
(eval/fidelity/humanizer-12b-Q8_0_fix-sizes.jsonl). Picked for readability among clean drafts; across
the whole set the model does make fact errors.
"""
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
EVAL = HERE.parent / "eval"
RUN = "humanizer-12b-Q8_0"      # the released file; eval/outputs/previous-12b-RLRt_* is the previous release
sys.path.insert(0, str(HERE))
import hz_text  # noqa: E402

PICKS = [
    ("en-email", "email_work_09_luna", "Work email", "英文工作邮件"),
    ("en-review", "review_product_02_luna", "Film review", "影评"),
    ("en-essay", "essay_student_00_glm", "Student essay", "学生作文"),
    ("en-forum", "forum_answer_04_luna", "Forum answer", "论坛回答"),
    ("zh-email", "zh_email_08_luna", "Chinese email to a landlord", "给房东的中文邮件"),
    ("zh-social", "zh_social_12_glm", "Chinese social post", "中文社交帖"),
]
WRITERS = {"glm": "GLM-5.3", "luna": "GPT-5.6 luna", "sonnet": "Claude Sonnet"}


def tidy(s: str) -> str:
    """Whitespace only: no spaces around line breaks; single line breaks become paragraph breaks
    when the text has no blank lines and no code (some samples were stored with ' \\n ')."""
    s = re.sub(r"[ \t]*\n[ \t]*", "\n", s.strip())
    if "\n\n" not in s and "```" not in s:
        s = s.replace("\n", "\n\n")
    return re.sub(r"\n{3,}", "\n\n", s)


def main():
    outputs, verdicts = {}, {}
    for half in "ab":
        for case, samples in json.load(open(EVAL / f"outputs/{RUN}_300{half}.json")).items():
            outputs[case] = (half, samples)
        for lang in ("en", "zh"):
            for r in json.load(open(EVAL / f"fidelity/{RUN}_{lang}_300{half}.json")):
                verdicts[(r["case"], r["i"])] = r["verdict"]
    needs_fix = {(r["case"], r["i"]) for r in map(json.loads, open(EVAL / f"fidelity/{RUN}_fix-sizes.jsonl"))}

    out = []
    for ex_id, case, genre_en, genre_zh in PICKS:
        half, samples = outputs[case]
        draft = tidy((EVAL / f"drafts/300{half}/{case}.txt").read_text(encoding="utf-8"))
        rewrite = tidy(samples[0]["text"])
        v = verdicts[(case, 0)] or {}
        clean = v.get("facts_all_kept") is True and not v.get("added_content") and (case, 0) not in needs_fix
        if not case.startswith("zh_"):
            clean = clean and not v.get("meaning_changed") and v.get("severity") == "none"
        assert clean, (case, v)
        nums = hz_text.numbers(draft)
        out.append({
            "id": ex_id,
            "case": case,
            "lang": "zh" if case.startswith("zh_") else "en",
            "genre": {"en": genre_en, "zh": genre_zh},
            "writer": WRITERS[case.rsplit("_", 1)[1]],
            "draft": draft,
            "output": rewrite,
            "words": [hz_text.count_words(draft), hz_text.count_words(rewrite)],
            "copy": round(hz_text.copy_rate(draft, rewrite), 2),
            "numbers": [len(nums) - len(hz_text.missing_numbers(draft, rewrite)), len(nums)],
            "judge_clean": clean,
        })
        print(f"{ex_id:10s} {case:24s} words {out[-1]['words']} copy {out[-1]['copy']} numbers {out[-1]['numbers']}")
    (HERE / "examples.json").write_text(json.dumps(out, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
