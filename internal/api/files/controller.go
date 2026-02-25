package files

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"io"
	"net/url"
	"strings"

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

	// 判断是系统模式还是用户模式
	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

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
		req.Path = c.Query("path")
	}

	if req.Path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	// 判断是系统模式还是用户模式
	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

	data, err := ctrl.fileService.GetDiskUsage(userID, req.Path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// CalculateSize 计算目录或文件大小
func (ctrl *Controller) CalculateSize(c *gin.Context) {
	userID := middleware.GetUserID(c)
	path := c.Query("path")

	if path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

	size, sizeStr, usage, err := ctrl.fileService.CalculateSize(userID, path, isRootMode)
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
	path := c.Query("path")

	if path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

	err := ctrl.fileService.DeleteFile(userID, path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}

// UploadFile 上传文件
func (ctrl *Controller) UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	targetPath := c.PostForm("path")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "文件上传失败")
		return
	}
	defer file.Close()

	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

	data, err := ctrl.fileService.UploadFile(userID, file, header, targetPath, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// DownloadFile 下载文件
func (ctrl *Controller) DownloadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	path := c.Query("path")

	if path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

	reader, filename, err := ctrl.fileService.DownloadFile(userID, path, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", "attachment; filename="+url.QueryEscape(filename))
	c.Header("Content-Type", "application/octet-stream")
	io.Copy(c.Writer, reader)
}

// UnzipFile 解压文件
func (ctrl *Controller) UnzipFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UnzipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode := strings.Contains(c.Request.URL.Path, "/system/")

	err := ctrl.fileService.UnzipFile(userID, &req, isRootMode)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, nil)
}
