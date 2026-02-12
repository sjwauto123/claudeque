package request

// QueueListRequest 获取分页排队队列列表请求体
type QueueListRequest struct {
	Id        int    `form:"id"`
	Name      string `form:"name"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
	PageSize  int    `form:"pageSize"`
	Page      int    `form:"page" binding:"min=1"`
}

type ReorderRequest struct {
	JobID       int `json:"job_id" binding:"required"`        // 被调整的任务ID
	TargetJobID int `json:"target_job_id" binding:"required"` // 目标任务ID：插到它前面
}
