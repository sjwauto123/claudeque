package response

import (
	"time"
)

type UserLogsResponse struct {
	Username    string    `json:"username"` // 添加用户名字段，便于查询
	RequestData string    `json:"request_data"`
	ActionType  string    `json:"action_type"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}
