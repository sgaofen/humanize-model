package launcher

// SysInfo 是选档和展示用的本机信息。
type SysInfo struct {
	RAMBytes uint64  `json:"ram_bytes"`
	RAMGB    float64 `json:"ram_gb"`
	CPU      string  `json:"cpu,omitempty"`
	GPU      GPUInfo `json:"gpu"`
	OS       string  `json:"os"`
	Arch     string  `json:"arch"`
}

type GPUInfo struct {
	NVIDIA bool     `json:"nvidia"`
	Vulkan bool     `json:"vulkan"`
	Names  []string `json:"names,omitempty"`
}

const gib = 1 << 30
