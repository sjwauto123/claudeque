package response

import "time"

type JobResponse struct {
	// 任务ID
	ID int64 `json:"id"`
	// 任务名称
	Name string `json:"name"`
	// 任务描述
	Description string `json:"description"`
	// 任务状态：0-待执行 1-排队中 2-执行中 3-已完成 4-失败 5-被终止 6-等待显卡
	Status int64 `json:"status"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 错误信息
	ResultMsg string `json:"result_msg"`
	// 使用的显卡（保留兼容性，实际应从其他接口获取）
	Card string `json:"card"`
	// 前方等待任务数（保留兼容性）
	Count int `json:"count"`
	// 已等待时间
	WaitTime string `json:"wait_time"`
}
