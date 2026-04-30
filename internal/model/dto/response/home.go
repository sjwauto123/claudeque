package response

import "time"

type HomeOverviewResponse struct {
	GpuSummary      HomeGpuSummary      `json:"gpu_summary"`
	Gpus            []HomeGPUOverview   `json:"gpus"`
	RunningJobs     []HomeRunningJob    `json:"-"`
	ServerProcesses []ServerProcessInfo `json:"server_processes"`
	QueueSummary    HomeQueueSummary    `json:"queue_summary"`
}

type HomeGpuSummary struct {
	Total int `json:"total"`
	Busy  int `json:"busy"`
	Idle  int `json:"idle"`
}

type HomeGPUOverview struct {
	ID           int    `json:"id"`
	Index        int    `json:"index"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Status       int    `json:"status"`
	CurrentJobID *int   `json:"current_job_id"`
	Temperature  int    `json:"temperature"`
	Utilization  int    `json:"utilization"`
	MemoryUsed   int    `json:"memory_used"`
	MemoryTotal  int    `json:"memory_total"`
}

type HomeRunningJob struct {
	JobID     int        `json:"job_id" gorm:"column:job_id"`
	JobName   string     `json:"job_name" gorm:"column:job_name"`
	UserID    int        `json:"user_id" gorm:"column:user_id"`
	UserName  string     `json:"user_name" gorm:"column:user_name"`
	GpuIDs    string     `json:"gpu_ids" gorm:"column:gpu_ids"`
	GpuNames  string     `json:"gpu_names" gorm:"column:gpu_names"`
	Status    int        `json:"status" gorm:"column:status"`
	StartedAt *time.Time `json:"started_at" gorm:"column:started_at"`
}

type HomeQueueSummary struct {
	Running    int `json:"running"`
	Queued     int `json:"queued"`
	WaitingGpu int `json:"waiting_gpu"`
}
