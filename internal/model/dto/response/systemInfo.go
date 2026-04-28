package response

// SystemProcessInfo 系统进程信息（Service 层使用）
type SystemProcessInfo struct {
	Username  string `json:"username"`
	PID       string `json:"pid"`
	JobName   string `json:"job_name"`
	GPUname   string `json:"gpu_name"`
	StartTime string `json:"start_time"`
	IsNormal  int    `json:"is_normal"`
	Command   string `json:"command"`
}

// ServerProcessInfo 服务器进程信息（Service 层使用）
type ServerProcessInfo struct {
	Username        string `json:"username"`
	PID             string `json:"pid"`
	JobName         string `json:"job_name"`
	GPUname         string `json:"gpu_name"`
	StartTime       string `json:"start_time"`
	IsNormal        int    `json:"is_normal"`
	Command         string `json:"command"`
	IsRetained      bool   `json:"is_retained"`
	RunningDuration int    `json:"running_duration"`
}

type GPUInfoResponse struct {
	Index      int    `json:"index"`
	DeviceName string `json:"name"`
	Temp       string `json:"temp"`
	Util       string `json:"util"`
	MemUsed    string `json:"mem_used"`
	MemTotal   string `json:"mem_total"`
}

type CpuInfoResponse struct {
	DeviceName string `json:"deviceName"`
	TotalCap   string `json:"totalCap"`
	UseCap     string `json:"useCap"`
	RemainCap  string `json:"remainCap"`
	Percentage string `json:"percentage"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	AutoTerminateEnabled bool `json:"auto_terminate_enabled"`
	MaxDurationMinutes   int  `json:"max_duration_minutes"`
}

type SystemInfosResponse struct {
	CpuList         []CpuInfoResponse   `json:"cpulist"`
	GpuList         []GPUInfoResponse   `json:"gpulist"`
	SystemProcesses []SystemProcessInfo `json:"system_processes"`
	ServerProcesses []ServerProcessInfo `json:"server_processes"`
}
