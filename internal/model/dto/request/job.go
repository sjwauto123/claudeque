package request

// JobListRequest 获取分页任务列表请求体
type JobListRequest struct {
	Id        int    `form:"id"`
	Status    int    `form:"status"`
	Name      string `form:"name"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
	PageSize  int    `form:"pageSize"`
	Page      int    `form:"page" binding:"min=1"`
}

// SubmitJobRequest 提交任务请求
type SubmitJobRequest struct {
	Name        string `json:"name" binding:"required"`      // 任务名称
	Description string `json:"description"`                  // 任务描述
	FilePath    string `json:"file_path" binding:"required"` // 训练脚本文件路径
	GpuIDs      []int  `json:"gpu_ids" binding:"required"`   // 使用显卡 ID 数组，如 [1,2]
	CondaEnv    string `json:"conda_env"`
}
