package files

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"io"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Controller 文件控制器
type Controller struct {
	fileService service.FileService
	logService  service.LogService
}

// NewController 创建文件控制器
func NewController(fileService service.FileService, logService service.LogService) *Controller {
	return &Controller{
		fileService: fileService,
		logService:  logService,
	}
}

// GetFileList 获取文件列表
// @Summary 获取文件列表
// @Description 获取指定目录下的文件和子目录列表
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param path query string false "目录路径"
// @Param keyword query string false "搜索关键词"
// @Param page query int true "页码"
// @Param page_size query int true "每页数量"
// @Success 200 {object} response.Response{data=dto.FilesListData}
// @Router /api/files/list [get]
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

	data, err := ctrl.fileService.GetFileList(userID, &req)
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

	data, err := ctrl.fileService.GetDiskUsage(userID, req.Path)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)
}

// CalculateSize 计算目录或文件大小
// @Summary 计算目录或文件大小
// @Description 计算指定目录或文件的大小
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param path query string true "路径"
// @Success 200 {object} response.Response{data=gin.H}
// @Router /api/files/size [get]
func (ctrl *Controller) CalculateSize(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	path := c.Query("path")
	if path == "" {
		response.BadRequest(c, "path required")
		return
	}

	bytes, str, usage, err := ctrl.fileService.CalculateSize(userID, path)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, gin.H{
		"bytes": bytes,
		"size":  str,
		"usage": usage,
	})
}

// DeleteFile 删除文件或目录
// @Summary 删除文件或目录
// @Description 删除指定的文件或目录
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.DeleteFileRequest true "删除参数"
// @Success 200 {object} response.Response
// @Router /api/files/delete [delete]
func (ctrl *Controller) DeleteFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.DeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.fileService.DeleteFile(userID, req.Path); err != nil {
		response.BizError(c, err)
		return
	}

	ctrl.logService.CreateLog(middleware.GetUsername(c), "DeleteFile", "Deleted: "+req.Path)
	response.Success(c, nil)
}

// Chmod 修改权限
// @Summary 修改文件或目录权限
// @Description 修改文件或目录的权限 (chmod)
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.ChmodRequest true "请求参数"
// @Success 200 {object} response.Response
// @Router /api/files/chmod [post]
func (ctrl *Controller) Chmod(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.ChmodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.fileService.Chmod(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	ctrl.logService.CreateLog(middleware.GetUsername(c), "Chmod", "Chmod "+req.Mode+": "+req.Path)
	response.Success(c, nil)
}

// UploadFile 上传文件
// @Summary 上传文件
// @Description 上传文件到指定目录
// @Tags 文件管理
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "上传的文件"
// @Param target_path formData string true "目标目录路径"
// @Success 200 {object} response.Response{data=dto.FileUploadData}
// @Router /api/files/upload_file [post]
func (ctrl *Controller) UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.UploadFileRequest
	if err := c.ShouldBind(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	file, err := req.File.Open()
	if err != nil {
		response.BadRequest(c, "打开上传文件失败")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := ctrl.fileService.UploadFile(userID, file, req.File, req.TargetPath)
	if err != nil {
		response.BizError(c, err)
		return
	}

	ctrl.logService.CreateLog(middleware.GetUsername(c), "UploadFile", "Uploaded: "+req.TargetPath+"/"+req.File.Filename)
	response.Success(c, data)
}

// DownloadFile 下载文件
// @Summary 下载文件
// @Description 下载指定文件
// @Tags 文件管理
// @Produce application/octet-stream
// @Security Bearer
// @Param path query string true "文件路径"
// @Success 200 {file} binary
// @Router /api/files/download [get]
func (ctrl *Controller) DownloadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.DownloadRequest
	// Spec says "Body request parameters" for GET.
	// But usually Download is GET with Query or POST with Body.
	// Spec says GET /api/files/download with Body.
	// Let's try to bind JSON, if fails, try Query (as common fallback).
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Path = c.Query("path")
	}

	if req.Path == "" {
		response.BadRequest(c, "缺少 path 参数")
		return
	}

	reader, filename, err := ctrl.fileService.DownloadFile(userID, req.Path)
	if err != nil {
		response.BizError(c, err)
		return
	}
	defer func(reader io.ReadCloser) {
		_ = reader.Close()
	}(reader)

	ctrl.logService.CreateLog(middleware.GetUsername(c), "DownloadFile", "Downloaded: "+req.Path)

	// Encode filename for header
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+encodedFilename)
	c.DataFromReader(200, -1, "application/octet-stream", reader, nil)
}

// UnzipFile 解压文件
// @Summary 解压文件
// @Description 解压指定的压缩文件
// @Tags 文件管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.UnzipRequest true "解压参数"
// @Success 200 {object} response.Response{data=dto.FileUnzipData}
// @Router /api/files/unzip [post]
func (ctrl *Controller) UnzipFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.UnzipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.fileService.UnzipFile(userID, &req); err != nil {
		response.BizError(c, err)
		return
	}

	ctrl.logService.CreateLog(middleware.GetUsername(c), "UnzipFile", "Unzip: "+req.Path+" to "+req.TargetPath)
	response.Success(c, nil)
}
