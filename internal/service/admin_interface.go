package service

import "cloudque/internal/model/dto/request"

type AdminService interface {
	// CreateUser 新建用户
	CreateUser(req *request.CreateRequest) error
	// DeleteUser 删除用户
	DeleteUser(id uint) error
	// AdminUpdateUser 管理员更新用户
	AdminUpdateUser(id uint, req *request.AdminUpdateUserRequest) error
}
