package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"strings"
)

type apiService struct {
	apiRepo repository.APIRepository
}

func NewAPIService(apiRepo repository.APIRepository) APIService {
	return &apiService{apiRepo: apiRepo}
}
func (s *apiService) PageList(req *request.APIPageQueryRequest) ([]*entity.Permission, int64, error) {
	// 参数校验和默认值设置
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 10
	}
	offset := (req.Page - 1) * req.PageSize

	// 调用Repository层查询数据
	return s.apiRepo.PageList(offset, req.PageSize, req.HTTPPath, req.Status)
}

func (s *apiService) Create(req *request.CreateAPIRequest) error {
	// 1. 参数校验
	if req.Name == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}
	if req.Slug == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}
	if req.HTTPMethod == "" {
		return errors.NewDefault(errors.CodeMissingParam)
	}

	// 2. 检查API名称是否重复
	exists, err := s.apiRepo.ExistsByName(req.Name)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查API名称是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "API名称已存在")
	}

	// 3. 检查API标识是否重复
	exists, err = s.apiRepo.ExistsBySlug(req.Slug)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查API标识是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "API标识已存在")
	}

	// 4. 检查HTTP路径是否重复（如果提供了路径）
	if req.HTTPPath != "" {
		exists, err = s.apiRepo.ExistsByHTTPPath(req.HTTPPath)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "检查HTTP路径是否重复失败", err)
		} else if exists {
			return errors.New(errors.CodeResourceAlreadyExists, "HTTP路径已存在")
		}
	}

	// 5. 构造实体对象并保存到数据库
	api := &entity.Permission{
		Name:       req.Name,
		Category:   req.Category,
		Slug:       req.Slug,
		Status:     *req.Status,
		HTTPMethod: req.HTTPMethod,
		HTTPPath:   req.HTTPPath,
		Sort:       req.Sort,
	}

	// 6. 保存到数据库
	err = s.apiRepo.Create(api)
	if err != nil {
		// 处理唯一约束冲突（兜底方案）
		if strings.Contains(err.Error(), "Duplicate entry") ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.NewDefault(errors.CodeResourceAlreadyExists)
		}
		return errors.NewWithErr(errors.CodeInternalError, "创建API失败!", err)
	}

	return nil
}

func (s *apiService) Update(req *request.UpdateAPIRequest) error {
	// 1. 先检查API是否存在
	existingAPI, err := s.apiRepo.GetAPIByID(req.ID)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询API信息失败", err)
	}
	if existingAPI == nil {
		return errors.New(errors.CodeResourceNotFound, "API不存在")
	}

	// 2. 构造更新对象
	api := &entity.Permission{
		BaseEntity: entity.BaseEntity{
			ID: req.ID,
		},
	}

	// 3. 更新非空字段
	if req.Name != "" {
		api.Name = req.Name
	}
	if req.Category != "" {
		api.Category = req.Category
	}
	if req.Slug != "" {
		api.Slug = req.Slug
	}
	if req.Status != nil {
		api.Status = *req.Status
	}
	if req.HTTPMethod != "" {
		api.HTTPMethod = req.HTTPMethod
	}
	if req.HTTPPath != "" {
		api.HTTPPath = req.HTTPPath
	}
	if req.Sort != nil {
		api.Sort = *req.Sort
	}

	// 4. 调用Repository层更新数据
	if err := s.apiRepo.Update(api); err != nil {
		// 处理唯一约束冲突
		if strings.Contains(err.Error(), "Duplicate entry") ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.NewDefault(errors.CodeResourceAlreadyExists)
		}
		return errors.NewWithErr(errors.CodeInternalError, "更新API失败!", err)
	}

	return nil
}

func (s *apiService) Delete(id int) error {
	// 先检查API是否存在
	existingAPI, err := s.apiRepo.GetAPIByID(id)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询API信息失败", err)
	}
	if existingAPI == nil {
		return errors.New(errors.CodeResourceNotFound, "API不存在")
	}

	// 调用Repository层删除数据
	return s.apiRepo.Delete(id)
}

func (s *apiService) BatchDelete(ids []int) error {
	// 参数校验
	if len(ids) == 0 {
		return errors.NewDefault(errors.CodeMissingParam)
	}

	// 检查所有API是否存在
	for _, id := range ids {
		existingAPI, err := s.apiRepo.GetAPIByID(id)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "查询API信息失败", err)
		}
		if existingAPI == nil {
			return errors.New(errors.CodeResourceNotFound, "API不存在")
		}
	}

	// 调用Repository层批量删除数据
	return s.apiRepo.BatchDelete(ids)
}

func (s *apiService) GetAPIByID(id int) (*entity.Permission, error) {
	return s.apiRepo.GetAPIByID(id)
}
