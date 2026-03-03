package response

import (
	"time"
)

type AdminLogResponse struct {
	Username   string    `json:"username"`
	ActionType string    `json:"action_type"`
	Status     int       `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}
