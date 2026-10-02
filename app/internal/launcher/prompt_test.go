package launcher

import "testing"

// 和 humanizer/promptfmt.py 的 fingerprint() 对齐;改了提示词这里必挂。
func TestPromptMatchesPython(t *testing.T) {
	if got := PromptFingerprint(); got != pythonFingerprint {
		t.Fatalf("提示词指纹 %s,promptfmt.py 是 %s", got, pythonFingerprint)
	}
}

func TestBuildPromptStrip(t *testing.T) {
	got := BuildPrompt(" \n\t\u3000\x1c Hello world.\u00a0\n\n")
	want := promptInstr + "\n\nHello world." + promptSep
	if got != want {
		t.Fatalf("strip 行为和 Python 不一致:\n%q", got)
	}
	// 中间的空白不动
	if BuildPrompt("a\n\n b") != promptInstr+"\n\na\n\n b"+promptSep {
		t.Fatal("不应改动正文内部空白")
	}
	// BOM 在 Python 里不是空白,不能被 strip
	if BuildPrompt("\ufeffx") != promptInstr+"\n\n\ufeffx"+promptSep {
		t.Fatal("BOM 不应被 strip")
	}
}
