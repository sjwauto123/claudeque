package files

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"io"
	"mime/multipart"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Controller 文件控制器
type Controller struct {
	fileService    service.FileService
	authService    service.AuthService
	userLogService service.UserOperationLogService
}

// NewController 创建文件控制器
func NewController(fileService service.FileService, authService service.AuthService, userLogService service.UserOperationLogService) *Controller {
	return &Controller{
		fileService:    fileService,
		authService:    authService,
		userLogService: userLogService,
	}
}

// GetFileList 获取文件列表

func (ctrl *Controller) GetFileList(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.FileListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权访问系统文件")
			return
		}
	}

	data, err := ctrl.fileService.GetFileList(userID, &req, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// GetDiskUsage 计算目录磁盘占比和大小
// @Summary 计算目录磁盘占比和大小
// @Description 计算指定目录的大小和磁盘占用比例
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.DiskUsageRequest true "请求参数"
// @Success 200 {object} response.Response{data=dto.DiskUsageData}
// @Router /api/user/directories/calculate-usage [get]
func (ctrl *Controller) GetDiskUsage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.DiskUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.Path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权获取系统磁盘使用情况")
			return
		}
	}

	data, err := ctrl.fileService.GetDiskUsage(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// ListHomeDirectories 获取 /home 目录下的所有用户目录列表
func (ctrl *Controller) ListHomeDirectories(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.FileListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	data, err := ctrl.fileService.GetHomeDirectoriesList(userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// CalculateSize 计算目录或文件大小
func (ctrl *Controller) CalculateSize(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DiskUsageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权计算系统文件大小")
			return
		}
	}

	size, sizeStr, usage, err := ctrl.fileService.CalculateSize(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"size":          size,
		"size_human":    sizeStr,
		"usage_percent": usage,
	})
}

// DeleteFile 删除文件或目录
func (ctrl *Controller) DeleteFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权删除系统文件")
			return
		}
	}

	err := ctrl.fileService.DeleteFile(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// UploadFile 上传文件
func (ctrl *Controller) UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UploadFileRequest
	if err := c.ShouldBind(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	file, err := req.File.Open()
	if err != nil {
		response.BadRequest(c, "文件打开失败: "+err.Error())
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)

	// 判断是系统模式还是用户模式
	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权上传系统文件")
			return
		}
	}

	data, err := ctrl.fileService.UploadFile(userID, file, req.File, req.TargetPath, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// DownloadFile 下载文件
func (ctrl *Controller) DownloadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DownloadRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权下载系统文件")
			return
		}
	}

	reader, filename, err := ctrl.fileService.DownloadFile(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			return
		}
	}(reader)

	c.Header("Content-Disposition", "attachment; filename="+url.QueryEscape(filename))
	c.Header("Content-Type", "application/octet-stream")
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		return
	}
}

// UnzipFile 解压文件
func (ctrl *Controller) UnzipFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UnzipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode := (req.Mode == request.System)

	if isRootMode {
		hasAccess, err := ctrl.authService.HasSystemAccess(userID)
		if err != nil {
			response.BizError(c, err)
			return
		}
		if !hasAccess {
			response.Forbidden(c, "无权解压系统文件")
			return
		}
	}

	err := ctrl.fileService.UnzipFile(userID, &req, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
