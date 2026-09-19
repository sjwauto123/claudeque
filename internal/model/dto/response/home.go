package response

type HomeOverviewResponse struct {
	GpuSummary      HomeGpuSummary      `json:"gpu_summary"`
	Gpus            []GPUInfoResponse   `json:"gpus"`
	ServerProcesses []ServerProcessInfo `json:"server_processes"`
	SystemProcesses []SystemProcessInfo `json:"system_processes"`
	QueueSummary    HomeQueueSummary    `json:"queue_summary"`
}

type HomeGpuSummary struct {
	Total int `json:"total"`
	Busy  int `json:"busy"`
	Idle  int `json:"idle"`
}

type HomeQueueSummary struct {
	Running    int `json:"running"`
	Queued     int `json:"queued"`
	WaitingGpu int `json:"waiting_gpu"`
}
