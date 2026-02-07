package request

type ReorderRequest struct {
	JobID       int `json:"job_id" binding:"required"`        // 被调整的任务ID
	TargetJobID int `json:"target_job_id" binding:"required"` // 目标任务ID：插到它前面
}
