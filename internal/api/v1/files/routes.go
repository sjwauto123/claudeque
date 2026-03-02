package files

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册文件路由
func (ctrl *Controller) RegisterRoutes(router *gin.RouterGroup) {

	// 文件管理接口
	fileGroup := router.Group("/files")
	fileGroup.Use(middleware.Auth())
	fileGroup.Use(middleware.RequirePermission(ctrl.authService))
	fileGroup.Use(middleware.UserOperationLogs(ctrl.userLogService))

	{
		// 文件管理
		fileGroup.GET("/list", middleware.WithOperation("获取文件列表"), ctrl.GetFileList)
		fileGroup.GET("/list/*path", middleware.WithOperation("获取文件列表"), ctrl.GetFileList)
		fileGroup.DELETE("/delete", middleware.WithOperation("删除文件"), ctrl.DeleteFile)
		fileGroup.POST("/upload", middleware.WithOperation("上传文件"), ctrl.UploadFile)
		fileGroup.GET("/upload/progress", middleware.WithOperation("获取上传进度"), ctrl.GetUploadProgress)
		fileGroup.GET("/download", middleware.WithOperation("下载文件"), ctrl.DownloadFile)
		fileGroup.POST("/unzip", middleware.WithOperation("解压文件"), ctrl.UnzipFile)
		fileGroup.GET("/size", middleware.WithOperation("计算文件大小"), ctrl.CalculateSize)
	}

	// 用户目录接口
	userDirGroup := router.Group("/directories")
	userDirGroup.Use(middleware.Auth())
	userDirGroup.Use(middleware.RequirePermission(ctrl.authService))
	userDirGroup.Use(middleware.UserOperationLogs(ctrl.userLogService))
	{
		// 计算目录磁盘占比和大小
		userDirGroup.GET("/calculate-usage", middleware.WithOperation("计算目录磁盘大小"), ctrl.GetDiskUsage)
		// 列出 /home 目录下的所有用户目录
		userDirGroup.GET("/list-home", middleware.WithOperation("列出所有用户目录"), ctrl.ListHomeDirectories)
		userDirGroup.GET("/list-home/*path", middleware.WithOperation("列出所有用户目录"), ctrl.ListHomeDirectories)
	}
}
