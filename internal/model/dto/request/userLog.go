package request

import "cloudque/pkg/response"

type UserLogsRequest struct {

	// 用户名
	Username string `form:"username" json:"username,omitempty"`
	// 操作类型
	ActionType string `form:"actionType" json:"actionType,omitempty"`
	// 结束时间
	EndTime string `form:"endTime" json:"endTime,omitempty"`

	// 开始时间
	StartTime string `form:"startTime" json:"startTime,omitempty"`

	response.PageRequest
}
