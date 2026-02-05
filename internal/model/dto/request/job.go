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
	Name        string `json:"name" binding:"required"`                  // 任务名称
	Description string `json:"description"`                              // 任务描述
	FilePath    string `json:"file_path" binding:"required"`             // 训练脚本文件路径
	GpuCount    int    `json:"gpu_count" binding:"required,min=1,max=4"` // 使用显卡数量
}
