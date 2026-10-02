package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func detectSys() SysInfo {
	s := SysInfo{OS: runtime.GOOS, Arch: runtime.GOARCH}
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&m))); r != 0 {
		s.RAMBytes = m.TotalPhys
	}
	s.RAMGB = float64(s.RAMBytes) / gib
	s.CPU = strings.TrimSpace(os.Getenv("PROCESSOR_IDENTIFIER"))

	sys32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	if os.Getenv("SystemRoot") == "" {
		sys32 = `C:\Windows\System32`
	}
	// NVIDIA 驱动一定会装 nvcuda.dll;各家显卡驱动一般都带 vulkan-1.dll
	_, errCuda := os.Stat(filepath.Join(sys32, "nvcuda.dll"))
	_, errVk := os.Stat(filepath.Join(sys32, "vulkan-1.dll"))
	s.GPU.NVIDIA = errCuda == nil
	s.GPU.Vulkan = errVk == nil
	if s.GPU.NVIDIA {
		cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if out, err := cmd.Output(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					s.GPU.Names = append(s.GPU.Names, line)
				}
			}
		}
	}
	return s
}
