package response

import "time"

type QueueJobResponse struct {
	JobID       uint      `json:"job_id"`
	UserName    string    `json:"user_name"`
	JobName     string    `json:"job_name"`
	Description string    `json:"description"`
	Status      int       `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
	WaitSeconds int64     `json:"wait_seconds"`
	FrontCount  int       `json:"front_count"`
}

type QueueJobDBRow struct {
	JobID       uint
	JobName     string
	Description string
	Status      int
	SubmittedAt time.Time
	UserName    string
}
