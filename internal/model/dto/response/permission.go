package response

import "time"

type Permission struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Slug       string    `json:"slug"`
	Type       string    `json:"type"`
	Status     int       `json:"status"`
	HttpMethod string    `json:"http_method"`
	HttpPath   string    `json:"http_path"`
	Sort       int       `json:"sort"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
