package service

type Scheduler interface {
	// Start 启动调度器
	Start()
	// Stop 停止调度器
	Stop()
	// TerminateAllRunningJobs 终止所有正在运行的任务并清理资源
	TerminateAllRunningJobs()
}
