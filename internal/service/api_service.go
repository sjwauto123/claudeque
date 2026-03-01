package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/pkg/errors"
	"strings"
	"time"
)

type apiService struct {
	apiRepo repository.APIRepository
}

func NewAPIService(apiRepo repository.APIRepository) APIService {
	return &apiService{apiRepo: apiRepo}
}

func (s *apiService) GetAPIByID(id int) (*entity.Permission, error) {
	return s.apiRepo.GetAPIByID(id)
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
	// 校验 http_path 格式
	if !strings.HasPrefix(req.HTTPPath, "/api") {
		return errors.New(errors.CodeInvalidParam, "路径必须以/api开头")
	}

	// 检查API标识是否重复
	exists, err := s.apiRepo.ExistsBySlug(req.Slug)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "检查API标识是否重复失败", err)
	} else if exists {
		return errors.New(errors.CodeResourceAlreadyExists, "API标识已存在")
	}

	// 检查 (method, path) 是否重复
	//exists, err = s.apiRepo.ExistsByMethodAndPath(req.HTTPMethod, req.HTTPPath)
	//if err != nil {
	//	return errors.NewWithErr(errors.CodeInternalError, "检查API路径是否重复失败", err)
	//} else if exists {
	//	return errors.New(errors.CodeResourceAlreadyExists, "该HTTP方法与路径的组合已存在")
	//}

	// 构造实体对象并保存到数据库
	api := &entity.Permission{
		Name:       req.Name,
		Category:   req.Category,
		Slug:       req.Slug,
		Status:     *req.Status,
		HttpMethod: req.HTTPMethod,
		HttpPath:   req.HTTPPath,
		Sort:       req.Sort,
	}

	// 保存到数据库
	err = s.apiRepo.Create(api)
	if err != nil {
		// 处理唯一约束冲突
		if strings.Contains(err.Error(), "Duplicate entry") ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errors.NewDefault(errors.CodeResourceAlreadyExists)
		}
		return errors.NewWithErr(errors.CodeInternalError, "创建API失败!", err)
	}

	return nil
}

func (s *apiService) Update(req *request.UpdateAPIRequest) error {
	// 根据id判断API是否存在
	existingAPI, err := s.apiRepo.GetAPIByID(req.ID)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "查询API信息失败", err)
	}
	if existingAPI == nil {
		return errors.New(errors.CodeResourceNotFound, "API不存在")
	}

	updates := make(map[string]interface{})

	if req.Slug != "" {
		exists, err := s.apiRepo.ExistsBySlug(req.Slug)
		if err != nil {
			return errors.NewWithErr(errors.CodeInternalError, "校验API标识失败", err)
		}
		if exists {
			return errors.New(errors.CodeResourceAlreadyExists, "API标识已存在")
		}
		updates["slug"] = req.Slug
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.Category != "" {
		updates["category"] = req.Category
	}

	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if req.HTTPMethod != "" {
		updates["http_method"] = req.HTTPMethod
	}

	if req.HTTPPath != "" {
		updates["http_path"] = req.HTTPPath
	}

	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}

	if len(updates) == 0 {
		return nil
	}

	updates["updated_at"] = time.Now()

	if err := s.apiRepo.Update(req.ID, updates); err != nil {
		return errors.NewWithErr(errors.CodeInternalError, "更新API失败", err)
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
	return s.apiRepo.BatchDelete(ids)
}
