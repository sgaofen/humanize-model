//go:build !windows

package launcher

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// 引擎放进独立进程组,关的时候连同它可能派生的子进程一起关。
func prepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func afterStart(cmd *exec.Cmd) {}

func terminate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

func forceKill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// killStaleEngine:上次启动器被强杀(kill -9、断电)时引擎可能还活着。
// 只有在那个 pid 确实还是 llama-server 时才杀,防止 pid 被复用误杀别的进程。
func killStaleEngine(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	comm := strings.TrimSpace(string(out))
	if !strings.HasSuffix(comm, "llama-server") && !strings.Contains(comm, "fakellama") {
		return false
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	return true
}

func freeDiskBytes(dir string) (uint64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return uint64(st.Bavail) * uint64(st.Bsize), true
}

func openURL(u string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", u).Start()
	}
	return exec.Command("xdg-open", u).Start()
}

func revealPath(p string) error { return openURL(p) }

// detach:新会话,脱离 .app 前台进程,前台退出后服务继续跑。
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
