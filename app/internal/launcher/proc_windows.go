package launcher

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const (
	createNoWindow             = 0x08000000
	jobObjectExtendedLimitInfo = 9
	jobLimitKillOnJobClose     = 0x2000
	processSetQuota            = 0x0100
	processTerminate           = 0x0001
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procGetDiskFreeSpaceExW      = kernel32.NewProc("GetDiskFreeSpaceExW")

	jobOnce   sync.Once
	jobHandle uintptr
)

type jobBasicLimit struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount, WriteOperationCount, OtherOperationCount uint64
	ReadTransferCount, WriteTransferCount, OtherTransferCount    uint64
}

type jobExtendedLimit struct {
	Basic                 jobBasicLimit
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// 所有引擎进程都挂到一个 "句柄关闭即全杀" 的 Job 上:
// 启动器不管怎么死(包括任务管理器强杀),llama-server 都会被系统一起带走。
func killOnCloseJob() uintptr {
	jobOnce.Do(func() {
		h, _, _ := procCreateJobObjectW.Call(0, 0)
		if h == 0 {
			return
		}
		var info jobExtendedLimit
		info.Basic.LimitFlags = jobLimitKillOnJobClose
		r, _, _ := procSetInformationJobObject.Call(h, jobObjectExtendedLimitInfo,
			uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
		if r == 0 {
			syscall.CloseHandle(syscall.Handle(h))
			return
		}
		jobHandle = h
	})
	return jobHandle
}

func prepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

func afterStart(cmd *exec.Cmd) {
	job := killOnCloseJob()
	if job == 0 || cmd.Process == nil {
		return
	}
	h, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(cmd.Process.Pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	procAssignProcessToJobObject.Call(job, uintptr(h))
}

// Windows 上没有对无控制台进程发 SIGTERM 的办法,直接结束。
func terminate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func forceKill(cmd *exec.Cmd) { terminate(cmd) }

// Windows 靠 Job 对象兜底,不会留下孤儿进程。
func killStaleEngine(pid int) bool { return false }

func freeDiskBytes(dir string) (uint64, bool) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var avail, total, free uint64
	r, _, _ := procGetDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if r == 0 {
		return 0, false
	}
	return avail, true
}

func openURL(u string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	return cmd.Start()
}

func revealPath(p string) error {
	return exec.Command("explorer", p).Start()
}

func detach(cmd *exec.Cmd) { prepareCmd(cmd) }
