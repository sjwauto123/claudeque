package entity

type Menu struct {
	BaseEntity
	ParentID int    `gorm:"type:int;comment:父菜单id" json:"parent_id"`
	Title    string `gorm:"type:varchar(50);not null;uniqueIndex;comment:菜单标题" json:"title"`
	Type     string `gorm:"type:varchar(50);comment:菜单类型: catalogue, menu" json:"type"`
	Sort     int    `gorm:"type:int;comment:排序值" json:"sort"`
	Status   int    `gorm:"type:int;not null;comment:状态: 0=停用, 1=启用" json:"status"`
	Icon     string `gorm:"type:varchar(50);comment:图标" json:"icon"`
	URI      string `gorm:"type:varchar(50);comment:路由路径" json:"uri"`
}

func (Menu) TableName() string {
	return "admin_menu"
}
