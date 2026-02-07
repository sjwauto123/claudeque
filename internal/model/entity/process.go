package entity

type Process struct {
	ID     int `json:"id" gorm:"primaryKey"`
	PID    int `json:"pid" gorm:"column:pid;uniqueIndex;not null"`
	CardID int `json:"card_id" gorm:"column:card_id;uniqueIndex;not null"`
	JobID  int `json:"job_id" gorm:"column:job_id;index;not null"`
}

func (Process) TableName() string {
	return "processes"
}
