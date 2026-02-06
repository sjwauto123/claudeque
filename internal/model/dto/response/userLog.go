package response

import (
	"time"
)

type UserLogsResponse struct {
	Username    string    `json:"username"` // 添加用户名字段，便于查询
	Object      string    `json:"object"`
	ActionType  string    `json:"actionType"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
