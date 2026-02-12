package entity

import "time"

type Process struct {
	ID        int        `json:"id" gorm:"primaryKey"`
	PID       int        `json:"pid" gorm:"column:pid;index;not null"`
	CardID    int        `json:"card_id" gorm:"column:card_id;index;not null"`
	JobID     int        `json:"job_id" gorm:"column:job_id;index;not null"`
	CreatedAt *time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	EndedAt   *time.Time `json:"ended_at" gorm:"column:ended_at"`
}

func (Process) TableName() string {
	return "processes"
}
