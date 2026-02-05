package entity

import "time"

// User 用户实体
type User struct {
	ID            uint      `gorm:"primaryKey;autoIncrement;comment:用户ID，主键" json:"id"`
	Username      string    `gorm:"type:varchar(190);not null;unique;comment:登录账号，唯一" json:"username"`
	Password      string    `gorm:"type:varchar(60);not null;comment:加密后的密码" json:"password"`
	Role          int8      `gorm:"type:tinyint;not null;default:1;comment:用户角色 1-普通用户 2-管理员" json:"role"`
	Avatar        *string   `gorm:"type:varchar(191);default:'';comment:头像URL" json:"avatar"`
	Email         *string   `gorm:"type:varchar(255);comment:邮箱" json:"email"`
	RememberToken *string   `gorm:"type:varchar(100);default:'';comment:记住我Token" json:"remember_token"`
	Status        int8      `gorm:"type:tinyint;not null;comment:账号状态" json:"status"`
	Priority      int8      `gorm:"type:tinyint;not null;default:1;comment:用户优先级 1-低 2-高" json:"priority"`
	MultiTraining int8      `gorm:"type:tinyint;not null;default:0;comment:多卡训练 0-否 1-是" json:"multi_training"`
	CrossServer   int8      `gorm:"type:tinyint;not null;default:0;comment:跨服务器调度 0-否 1-是" json:"cross_server"`
	HomeDir       *string   `gorm:"type:varchar(255);comment:Linux Home 目录" json:"home_dir"`
	CreatedAt     time.Time `gorm:"type:datetime(3);default:CURRENT_TIMESTAMP(3);comment:创建时间" json:"created_at"`
	UpdatedAt     time.Time `gorm:"type:datetime(3);default:CURRENT_TIMESTAMP(3);comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
