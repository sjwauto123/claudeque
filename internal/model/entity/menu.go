package entity

type Menu struct {
	BaseEntity
	ParentID int    `gorm:"type:int;default:0" json:"parent_id"`
	Title    string `gorm:"type:varchar(100);not null" json:"title"`
	Status   int    `gorm:"type:int;default:1" json:"status"`
	Type     string `gorm:"type:varchar(20);default:''" json:"type"`
	Icon     string `gorm:"type:varchar(100);default:''" json:"icon"`
	URI      string `gorm:"type:varchar(255);default:''" json:"uri"`
	Sort     int    `gorm:"type:int;default:0" json:"sort"`
}

func (Menu) TableName() string {
	return "admin_menu"
}
