package entity

type Process struct {
	ID     uint `json:"id" gorm:"primaryKey"`
	PID    int  `json:"pid" gorm:"column:pid;uniqueIndex;not null"`
	CardID uint `json:"card_id" gorm:"column:card_id;uniqueIndex;not null"`
	JobID  uint `json:"job_id" gorm:"column:job_id;index;not null"`
}

func (Process) TableName() string {
	return "processes"
}
