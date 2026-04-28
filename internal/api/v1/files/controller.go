package files

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"path"
	"strconv"
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
		logger.Warn("GetFileList: 用户未登录")
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.FileListRequest
	// 1. 尝试从 Query 参数获取 (标准 GET 请求)
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Errorf("GetFileList: 参数绑定失败: %v", err)
		response.BadRequest(c, err.Error())
		return
	}

	// 2. 兼容性：支持从路径参数获取 path (解决前端路径传参问题)
	if pathParam := c.Param("path"); pathParam != "" {
		// 如果 pathParam 是 /*path 形式，可能会带前缀斜杠，需要处理
		req.Path = path.Clean(pathParam)
	}

	// 3. 兼容性：如果 PageSize 为空，尝试从 pageSize (小驼峰) 获取
	if req.PageSize == 0 {
		if ps := c.Query("pageSize"); ps != "" {
			if v, _ := strconv.Atoi(ps); v > 0 {
				req.PageSize = v
			}
		}
	}
	// 4. 如果依然没有任何核心参数，尝试从 JSON Body 获取 (兼容前端非标准调用)
	if req.Path == "" && req.Page == 0 && req.PageSize == 0 {
		_ = c.ShouldBindJSON(&req)
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("GetFileList: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.GetFileList(userID, &req, isRootMode)
	if err != nil {
		logger.Errorf("GetFileList: 获取文件列表失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
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
// @Param path query string true "目录路径"
// @Success 200 {object} response.Response{data=dto.DiskUsageData}
// @Router /api/user/directories/calculate-usage [get]
func (ctrl *Controller) GetDiskUsage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		logger.Warn("GetDiskUsage: 用户未登录")
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.DiskUsageRequest
	// 优先尝试从 Query 参数获取 (标准 GET 请求)
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 绑定失败，尝试从 JSON Body 获取
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			logger.Errorf("GetDiskUsage: 参数绑定失败: %v", err)
			response.BadRequest(c, err.Error())
			return
		}
	}
	// 二次检查：如果 Query 绑定成功但 Path 为空，尝试 JSON
	if req.Path == "" {
		_ = c.ShouldBindJSON(&req)
	}

	if req.Path == "" {
		logger.Warn("GetDiskUsage: 缺少 path 参数")
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("GetDiskUsage: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.GetDiskUsage(userID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("GetDiskUsage: 获取磁盘使用情况失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// ListHomeDirectories 获取 /home 目录下的所有用户目录列表
func (ctrl *Controller) ListHomeDirectories(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		logger.Warn("ListHomeDirectories: 用户未登录")
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.FileListRequest
	// 1. 尝试从 Query 参数获取 (标准 GET 请求)
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Errorf("ListHomeDirectories: 参数绑定失败: %v", err)
		response.BadRequest(c, err.Error())
		return
	}

	// 2. 兼容性：支持从路径参数获取 path
	if pathParam := c.Param("path"); pathParam != "" {
		req.Path = path.Clean(pathParam)
	}

	// 3. 兼容性：如果 PageSize 为空，尝试从 pageSize (小驼峰) 获取
	if req.PageSize == 0 {
		if ps := c.Query("pageSize"); ps != "" {
			if v, _ := strconv.Atoi(ps); v > 0 {
				req.PageSize = v
			}
		}
	}
	// 4. 如果依然没有任何核心参数，尝试从 JSON Body 获取 (兼容前端非标准调用)
	if req.Path == "" && req.Page == 0 && req.PageSize == 0 {
		_ = c.ShouldBindJSON(&req)
	}

	data, err := ctrl.fileService.GetHomeDirectoriesList(userID, &req, true)
	if err != nil {
		logger.Errorf("ListHomeDirectories: 获取家目录列表失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// CalculateSize 计算目录或文件大小
func (ctrl *Controller) CalculateSize(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DiskUsageRequest
	// 优先尝试从 Query 参数获取
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 绑定失败，尝试从 JSON Body 获取
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			logger.Errorf("CalculateSize: 参数绑定失败: %v", err)
			response.BadRequest(c, err.Error())
			return
		}
	}
	// 二次检查：如果 Query 绑定成功但 Path 为空，尝试 JSON
	if req.Path == "" {
		_ = c.ShouldBindJSON(&req)
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("CalculateSize: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}

	size, sizeStr, usage, err := ctrl.fileService.CalculateSize(userID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("CalculateSize: 计算大小失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
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
		logger.Errorf("DeleteFile: 参数绑定失败: %v", err)
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("DeleteFile: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}

	err = ctrl.fileService.DeleteFile(userID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("DeleteFile: 删除文件失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
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
		logger.Errorf("UploadFile: 参数绑定失败: %v", err)
		response.BadRequest(c, err.Error())
		return
	}

	file, err := req.File.Open()
	if err != nil {
		logger.Errorf("UploadFile: 文件打开失败: %v", err)
		response.BadRequest(c, "文件打开失败: "+err.Error())
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			logger.Errorf("UploadFile: 关闭上传文件失败: %v", err)
			return
		}
	}(file)

	// 判断是系统模式还是用户模式
	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("UploadFile: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}

	data, err := ctrl.fileService.UploadFile(userID, file, req.File, req.TargetPath, isRootMode)
	if err != nil {
		// 如果是上传锁冲突，返回特定状态码给前端
		if strings.Contains(err.Error(), "当前有正在进行的上传任务") {
			logger.Warnf("UploadFile: 上传锁冲突: userID=%d, path=%s", userID, req.TargetPath)
			// 429 Too Many Requests 或者 409 Conflict
			response.Error(c, 409, err.Error())
			return
		}
		logger.Errorf("UploadFile: 上传文件失败: userID=%d, path=%s, err=%v", userID, req.TargetPath, err)
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// GetUploadProgress 获取上传进度
func (ctrl *Controller) GetUploadProgress(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UploadProgressRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Errorf("GetUploadProgress: 参数绑定失败: %v", err)
		response.BadRequest(c, err.Error())
		return
	}

	data, err := ctrl.fileService.GetUploadProgress(userID, req.Filename, req.TargetPath)
	if err != nil {
		logger.Errorf("GetUploadProgress: 获取上传进度失败: userID=%d, filename=%s, path=%s, err=%v", userID, req.Filename, req.TargetPath, err)
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// DownloadFile 下载文件
func (ctrl *Controller) DownloadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.DownloadRequest
	// 优先尝试从 Query 参数获取 (支持浏览器直接下载)
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 Query 参数绑定失败(通常是校验失败)，或者没有传参
		// 尝试从 JSON Body 获取 (支持前端 POST/GET JSON 调用)
		// 注意：GET 请求带 Body 不符合标准，但在某些内部调用中可能存在
		if errBody := c.ShouldBindJSON(&req); errBody != nil {
			logger.Errorf("DownloadFile: 参数绑定失败: %v", err)
			// 如果两者都失败，返回 Query 的错误(或者根据情况返回)
			response.BadRequest(c, "Invalid parameters: "+err.Error())
			return
		}
	}
	// 二次校验：如果 Query 绑定成功但 Path 为空(虽然有 required 校验，但为了稳妥)，再次尝试 JSON
	if req.Path == "" {
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Errorf("DownloadFile: 缺少 path 参数: %v", err)
			response.BadRequest(c, err.Error())
			return
		}
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("DownloadFile: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}
	reader, filename, fileSize, _, err := ctrl.fileService.DownloadFile(userID, req.Path, isRootMode)
	if err != nil {
		logger.Errorf("DownloadFile: 下载文件失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
		response.BizError(c, err)
		return
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			logger.Errorf("DownloadFile: 关闭下载文件失败: %v", err)
			return
		}
	}(reader)

	// 设置响应头
	c.Header("Content-Disposition", "attachment; filename="+url.QueryEscape(filename))
	c.Header("Content-Type", "application/octet-stream") // 通用二进制类型
	c.Header("Content-Length", fmt.Sprintf("%d", fileSize))

	// 使用 http.ServeContent 进行流式传输，它能处理 Range 请求等
	// 注意：SFTP reader 不支持 Seek，所以我们不能直接用 http.ServeContent
	// 我们手动设置了必要的头，然后用 io.Copy
	// 为了提升性能，可以使用 io.CopyBuffer
	buf := make([]byte, 32*1024) // 32KB buffer
	_, err = io.CopyBuffer(c.Writer, reader, buf)
	if err != nil {
		// 记录错误，但此时可能已经无法向客户端发送错误信息了
		logger.Errorf("下载文件时发生错误: %v", err)
	}
}

// UnzipFile 解压文件 (异步)
func (ctrl *Controller) UnzipFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req request.UnzipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf("UnzipFile: 参数绑定失败: %v", err)
		response.BadRequest(c, err.Error())
		return
	}

	isRootMode, err := ctrl.authService.HasSystemAccess(userID, service.AccessTypeFile)
	if err != nil {
		logger.Errorf("UnzipFile: 检查系统访问权限失败: userID=%d, err=%v", userID, err)
		response.BizError(c, err)
		return
	}

	// 同步执行解压，以便立即返回错误（如文件已存在）
	// 注意：如果解压文件过大，可能会导致请求超时，建议后续优化为 预检查+异步任务 模式
	err = ctrl.fileService.UnzipFile(userID, &req, isRootMode)
	if err != nil {
		logger.Errorf("UnzipFile: 解压文件失败: userID=%d, path=%s, err=%v", userID, req.Path, err)
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"message": "解压任务已在后台开始执行",
	})
}

// GetAllUsersDiskUsage 获取所有用户磁盘使用情况
// @Summary 获取所有用户磁盘使用情况
// @Description 获取所有用户磁盘使用情况列表（管理员接口）
// @Tags 文件管理
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]response.UserDiskUsageData}
// @Router /api/admin/disk-usage/all [get]
func (ctrl *Controller) GetAllUsersDiskUsage(c *gin.Context) {
	// 权限检查由 middleware.RequirePermission 处理
	data, err := ctrl.fileService.GetAllUsersDiskUsage()
	if err != nil {
		logger.Errorf("GetAllUsersDiskUsage: 获取所有用户磁盘使用情况失败: err=%v", err)
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}
