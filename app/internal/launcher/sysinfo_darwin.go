package launcher

import (
	"encoding/binary"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func detectSys() SysInfo {
	s := SysInfo{OS: runtime.GOOS, Arch: runtime.GOARCH}
	s.RAMBytes = sysctlUint64("hw.memsize")
	if s.RAMBytes == 0 { // 兜底:命令行
		if out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output(); err == nil {
			s.RAMBytes, _ = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
		}
	}
	s.RAMGB = float64(s.RAMBytes) / gib
	if v, err := syscall.Sysctl("machdep.cpu.brand_string"); err == nil {
		s.CPU = strings.TrimSpace(v)
	}
	// Apple 芯片统一内存,Metal 一定在
	s.GPU = GPUInfo{Names: []string{"Metal"}}
	return s
}

// syscall.Sysctl 返回原始字节,而且会吃掉末尾一个 0 字节,补回 8 字节再按小端解。
func sysctlUint64(name string) uint64 {
	v, err := syscall.Sysctl(name)
	if err != nil {
		return 0
	}
	b := []byte(v)
	if len(b) > 8 {
		return 0
	}
	for len(b) < 8 {
		b = append(b, 0)
	}
	return binary.LittleEndian.Uint64(b)
}
