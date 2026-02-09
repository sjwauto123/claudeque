package response

type AdminLogResponse struct {
	Username string `json:"username"`
	// 操作类型 (Login, SubmitJob, CancelJob, DeleteFile...)
	ActionType string `json:"actionType"`
	// 创建时间
	CreatedAt string `json:"created_at"`
	// 操作对象
	Object string `json:"object"`
	// 操作用户名
}
