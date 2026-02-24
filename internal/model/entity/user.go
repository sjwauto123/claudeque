package entity

// User 用户实体
type User struct {
	BaseEntity
	Username      string `gorm:"type:varchar(50);uniqueIndex;not null;comment:登录账号" json:"username"`
	Password      string `gorm:"type:varchar(255);not null;comment:加密后的密码" json:"-"`
	Avatar        string `gorm:"type:longtext;comment:用户头像(Base64格式)" json:"avatar"`
	Email         string `gorm:"type:varchar(100);comment:邮箱" json:"email"`
	Status        int    `gorm:"type:int;default:1;comment:账号状态" json:"status"`
	Priority      int    `gorm:"type:int;default:1;comment:用户优先级" json:"priority"`
	MultiTraining int    `gorm:"type:int;default:0;comment:多卡训练" json:"multi_training"`
	CrossServer   int    `gorm:"type:int;default:0;comment:跨服务器调度" json:"cross_server"`

	Roles []Role `gorm:"many2many:admin_role_users;joinForeignKey:user_id;JoinReferences:role_id" json:"roles"`
}

// TableName 指定表名
func (User) TableName() string {
	return "admin_users"
}
