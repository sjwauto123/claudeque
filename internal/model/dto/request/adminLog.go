package request

import "cloudque/pkg/response"

type AdminLogsRequest struct {
	KeyWord string `form:"key_word"`
	response.PageRequest
}
