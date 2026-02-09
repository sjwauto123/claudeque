package entity

type Menu struct {
	BaseEntity
	ParentID *int   `gorm:"column:parent_id;size:10" json:"parent_id,omitempty"` // 可为空，根菜单为 nil
	Title    string `gorm:"column:title;size:50;not null" json:"title"`
	Type     string `gorm:"column:type;size:50" json:"type"` // 菜单类型: catalogue, menu, button
	Sort     int    `gorm:"column:sort;size:11" json:"sort"`
	Status   int    `gorm:"column:status;size:4;not null" json:"status"` // 0=停用, 1=启用
	Icon     string `gorm:"column:icon;size:50" json:"icon"`
	URI      string `gorm:"column:uri;size:50" json:"uri"` // 路由路径，如 "/dashboard"
}

func (Menu) TableName() string {
	return "admin_menu"
}
