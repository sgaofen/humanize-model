package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// 提示词必须和训练时一字不差。事实源是仓库里的 humanizer/promptfmt.py;
// 这里是逐字拷贝,prompt_test.go 用 promptfmt.fingerprint() 的值把它钉死。
//
// 网页端(web/js/prompt.js)负责真正拼提示词,它从 /app/config 拿到下面两段字符串,
// 并在启动时用 probe(= BuildPrompt("X"))自检:拼出来不一样就拒绝改写。
const (
	promptInstr = "Rewrite the text below so it reads like a person wrote it, not a language model.\n" +
		"\n" +
		"Reorganize it as you see fit. Vary sentence length on purpose. Cut hedging,\n" +
		"throat-clearing, and any sentence that only announces what comes next.\n" +
		"Prefer the concrete word over the abstract one. It is fine to sound uneven.\n" +
		"\n" +
		"Every fact, number, unit, date, name and quotation must survive unchanged."
	promptSep = "\n\n### Rewritten:\n\n"

	// promptfmt.py 的 fingerprint():sha256(build_prompt('X'))[:16]
	pythonFingerprint = "cc51d66b4c593fbe"
)

// BuildPrompt = INSTR + "\n\n" + draft.strip() + "\n\n### Rewritten:\n\n"
func BuildPrompt(draft string) string {
	return promptInstr + "\n\n" + pyStrip(draft) + promptSep
}

func PromptFingerprint() string {
	sum := sha256.Sum256([]byte(BuildPrompt("X")))
	return hex.EncodeToString(sum[:])[:16]
}

// pyStrip 复刻 Python str.strip():Python 的空白比 Go 的 unicode.IsSpace
// 多了 \x1c-\x1f 四个分隔符。
func pyStrip(s string) string {
	return strings.TrimFunc(s, isPySpace)
}

func isPySpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}
