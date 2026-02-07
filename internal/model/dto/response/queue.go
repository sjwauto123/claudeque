package response

import "time"

type QueueJobResponse struct {
	JobID       int       `json:"job_id"`
	UserName    string    `json:"user_name"`
	JobName     string    `json:"job_name"`
	Description string    `json:"description"`
	Status      int       `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
	WaitSeconds int       `json:"wait_seconds"`
	FrontCount  int       `json:"front_count"`
}

type QueueJobDBRow struct {
	JobID       int
	JobName     string
	Description string
	Status      int
	SubmittedAt time.Time
	UserName    string
}
