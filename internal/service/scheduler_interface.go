package service

import "cloudque/internal/model/entity"

type Scheduler interface {
	Start()
	Stop()
	TerminateAllRunningJobs()
	Run()
	ProcessQueue()
	ExecuteJob(job *entity.Job) error
	MonitorJob(jp *JobProcess)
}
