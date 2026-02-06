package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	bizerrors "cloudque/pkg/errors"

	"golang.org/x/crypto/bcrypt"
)

func (s *userService) CreateUser(req *request.CreateRequest) error {
	exists, err := s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.ErrUserAlreadyExists
	}

	exists, err = s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &entity.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Status:   req.Status,
	}
	if err := s.userRepo.Create(user); err != nil {
		return err
	}
	// 默认赋予普通用户角色
	if err := s.userRepo.AssignRoleByName(user.ID, "user"); err != nil {
		return err
	}
	return nil
}

func (s *userService) DeleteUser(id uint) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}
	if user == nil {
		return bizerrors.ErrUserNotFound
	}
	if err := s.userRepo.ClearRoles(id); err != nil {
		return err
	}
	return s.userRepo.Delete(id)
}

func (s *userService) AdminUpdateUser(id uint, req *request.AdminUpdateUserRequest) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}
	if user == nil {
		return bizerrors.ErrUserNotFound
	}

	if req.Username != "" {
		existing, err := s.userRepo.FindByUsername(req.Username)
		if err != nil {
			return err
		}
		if existing != nil && existing.ID != id {
			return bizerrors.ErrUserAlreadyExists
		}
		user.Username = req.Username
	}

	if req.Email != "" {
		existing, err := s.userRepo.FindByEmail(req.Email)
		if err != nil {
			return err
		}
		if existing != nil && existing.ID != id {
			return bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
		}
		user.Email = req.Email
	}

	if req.Status != nil {
		user.Status = *req.Status
	}

	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}

	if req.Priority != nil {
		user.Priority = *req.Priority
	}

	if req.MultiTraining != nil {
		user.MultiTraining = *req.MultiTraining
	}

	if req.CrossServer != nil {
		user.CrossServer = *req.CrossServer
	}

	if req.Roles != nil {
		if err := s.userRepo.ReplaceRolesByNames(id, *req.Roles); err != nil {
			return err
		}
	}
	return s.userRepo.Update(user)
}
