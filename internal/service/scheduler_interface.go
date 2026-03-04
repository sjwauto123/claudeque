package service

type Scheduler interface {
	// Start 启动调度器
	Start()
	// Stop 停止调度器
	Stop()
}
