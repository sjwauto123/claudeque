package response

type ProcessInfoResponse struct {
	Username  string `json:"username"`
	PID       string `json:"pid"`
	GPUname   string `json:"gpu_name"`
	StartTime string `json:"start_time"`
	IsNormal  int    `json:"is_normal"`
	Runtime   string `json:"runtime"`
	Command   string `json:"command"`
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
type SystemInfosResponse struct {
	CpuList     []CpuInfoResponse     `json:"cpulist"`
	GpuList     []GPUInfoResponse     `json:"gpulist"`
	ProcessList []ProcessInfoResponse `json:"processlist"`
}
