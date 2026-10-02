#!/usr/bin/env python3
"""Build examples.json for the Space from the public evaluation set in ../eval.

    python3 space/build_examples.py

Each example is a held-out draft and the released 12B model's FIRST sample for it, unedited
(eval/outputs/humanizer-12b_300{a,b}.json, index 0). Only whitespace is normalised for display.
The fact-judge verdict for that same sample comes from eval/fidelity/. Picked for readability among
drafts whose verdict is clean; across the whole set the model does make fact errors.
"""
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
EVAL = HERE.parent / "eval"
sys.path.insert(0, str(HERE))
import hz_text  # noqa: E402

PICKS = [
    ("en-email", "email_work_00_luna", "Work email", "英文工作邮件"),
    ("en-reddit", "reddit_post_10_sonnet", "Reddit post", "Reddit 帖"),
    ("en-essay", "essay_student_00_glm", "Student essay", "学生作文"),
    ("en-forum", "forum_answer_07_luna", "Forum answer with code", "带代码的论坛回答"),
    ("zh-email", "zh_email_15_sonnet", "Chinese work email", "中文工作邮件"),
    ("zh-social", "zh_social_09_glm", "Chinese social post", "中文社交帖"),
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
        for case, samples in json.load(open(EVAL / f"outputs/humanizer-12b_300{half}.json")).items():
            outputs[case] = (half, samples)
        for lang in ("en", "zh"):
            for r in json.load(open(EVAL / f"fidelity/humanizer-12b_{lang}_300{half}.json")):
                verdicts[(r["case"], r["i"])] = r["verdict"]

    out = []
    for ex_id, case, genre_en, genre_zh in PICKS:
        half, samples = outputs[case]
        draft = tidy((EVAL / f"drafts/300{half}/{case}.txt").read_text(encoding="utf-8"))
        rewrite = tidy(samples[0]["text"])
        v = verdicts[(case, 0)]
        clean = bool(v.get("facts_all_kept")) and not v.get("added_content") and not v.get("meaning_changed") \
            and v.get("severity", "none") in ("none", None)
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
