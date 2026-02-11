package request

import "cloudque/pkg/response"

type UserLogsRequest struct {

	// 用户名
	Username string `form:"username" json:"username,omitempty"`
	// 操作类型
	ActionType string `form:"action_type" json:"action_type,omitempty"`
	// 结束时间
	EndTime string `form:"end_time" json:"end_time,omitempty"`

	// 开始时间
	StartTime string `form:"start_time" json:"start_time,omitempty"`

	response.PageRequest
}
