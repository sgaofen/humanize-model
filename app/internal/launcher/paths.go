package launcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// defaultDataDir:Mac ~/Library/Application Support/Humanizer,
// Windows %LOCALAPPDATA%\Humanizer,其他系统 $XDG_DATA_HOME/Humanizer。
func defaultDataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Humanizer")
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "Humanizer")
		}
		return filepath.Join(home, "AppData", "Local", "Humanizer")
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, "Humanizer")
		}
		return filepath.Join(home, ".local", "share", "Humanizer")
	}
}

// Backend 是一个可以尝试的 llama-server 二进制。
type Backend struct {
	Name string `json:"name"` // metal / cuda / vulkan / cpu / custom
	Path string `json:"path"`
	GPU  bool   `json:"gpu"`
	// CPU 模式下额外加的参数(Mac 上用同一个二进制跑纯 CPU 时要 --device none)
	Extra []string `json:"-"`
}

// discoverBackends 找随包附带的引擎,按优先级排好。
//
//	macOS  .app:  Humanizer.app/Contents/MacOS/Humanizer
//	              Humanizer.app/Contents/Resources/engine/metal/llama-server
//	Windows:      安装目录\Humanizer.exe
//	              安装目录\engine\{cuda,vulkan,cpu}\llama-server.exe
//	开发/便携:    启动器同目录下的 engine/<name>/llama-server
func discoverBackends(overrides []string, gpu GPUInfo) []Backend {
	if len(overrides) > 0 {
		return parseEngineOverrides(overrides)
	}
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	dir := filepath.Dir(exe)
	roots := []string{
		filepath.Join(dir, "..", "Resources", "engine"), // .app 里
		filepath.Join(dir, "engine"),                    // Windows 安装目录 / 便携版
	}
	bin := "llama-server"
	if runtime.GOOS == "windows" {
		bin = "llama-server.exe"
	}
	find := func(name string) string {
		for _, r := range roots {
			p := filepath.Join(r, name, bin)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return filepath.Clean(p)
			}
		}
		return ""
	}

	var out []Backend
	switch runtime.GOOS {
	case "darwin":
		if p := find("metal"); p != "" {
			out = append(out, Backend{Name: "metal", Path: p, GPU: true})
			// Metal 起不来时同一个二进制退回纯 CPU
			out = append(out, Backend{Name: "cpu", Path: p, GPU: false, Extra: []string{"--device", "none"}})
		}
	case "windows":
		// 有 NVIDIA 驱动才试 CUDA;有 Vulkan 运行时才试 Vulkan;CPU 兜底。
		if gpu.NVIDIA {
			if p := find("cuda"); p != "" {
				out = append(out, Backend{Name: "cuda", Path: p, GPU: true})
			}
		}
		if gpu.Vulkan {
			if p := find("vulkan"); p != "" {
				out = append(out, Backend{Name: "vulkan", Path: p, GPU: true})
			}
		}
		if p := find("cpu"); p != "" {
			out = append(out, Backend{Name: "cpu", Path: p, GPU: false})
		}
	default:
		for _, n := range []string{"cuda", "vulkan", "cpu"} {
			if p := find(n); p != "" {
				out = append(out, Backend{Name: n, Path: p, GPU: n != "cpu"})
			}
		}
	}
	return out
}

// parseEngineOverrides:开发时用 --engine / HUMANIZER_ENGINE 指定引擎。
// 形如 "/path/llama-server" 或 "cuda=/a;cpu=/b"(多个用 ; 分隔,也可以重复 --engine)。
func parseEngineOverrides(items []string) []Backend {
	var out []Backend
	for _, it := range items {
		for _, part := range strings.Split(it, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, path := "custom", part
			if i := strings.Index(part, "="); i > 0 && !strings.ContainsAny(part[:i], `/\`) {
				name, path = part[:i], part[i+1:]
			}
			out = append(out, Backend{Name: name, Path: path, GPU: name != "cpu"})
		}
	}
	return out
}
