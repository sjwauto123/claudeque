package request

import "cloudque/pkg/response"

type AdminLogsRequest struct {
	KeyWord string `form:"key_word"`
	// 状态：success 或 failed
	Status string `form:"status" json:"status,omitempty"`
	response.PageRequest
}
