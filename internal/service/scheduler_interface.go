package service

import "cloudque/internal/model/entity"

type Scheduler interface {
	// Start 启动调度器
	Start()
	// Stop 停止调度器
	Stop()
	// TerminateAllRunningJobs 终止所有正在运行的任务并清理资源
	TerminateAllRunningJobs()
	// Run 调度器主循环
	Run()
	// ProcessQueue 处理排队队列
	ProcessQueue()
	// ExecuteJob 执行任务
	ExecuteJob(job *entity.Job) error
	// MonitorJob 监控任务执行状态
	MonitorJob(jp *JobProcess)
}
