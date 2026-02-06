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
	// 使用的显卡
	Card string `json:"card"`
	// 前方等待任务数
	Count int `json:"count"`
	// 已等待时间
	WaitTime string `json:"wait_time"`
}

type JobStatsResponse struct {
	Total     int64 `json:"total"`
	Running   int64 `json:"running"`
	Queued    int64 `json:"queued"`
	Exception int64 `json:"exception"`
}
