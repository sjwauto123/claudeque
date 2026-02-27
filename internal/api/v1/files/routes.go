package files

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册文件路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 文件管理接口
	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	r.Use(middleware.UserOperationLogs(ctrl.userLogService))
	router := r.Group("/files")
	{
		// 文件管理
		router.GET("/list", middleware.WithOperation("获取文件列表"), ctrl.GetFileList)
		router.DELETE("/delete", middleware.WithOperation("删除文件"), ctrl.DeleteFile)
		router.POST("/upload", middleware.WithOperation("上传文件"), ctrl.UploadFile)
		router.GET("/download", middleware.WithOperation("下载文件"), ctrl.DownloadFile)
		router.POST("/unzip", middleware.WithOperation("解压文件"), ctrl.UnzipFile)
		router.GET("/size", middleware.WithOperation("计算文件大小"), ctrl.CalculateSize)
	}

	// 用户目录接口
	userDirGroup := router.Group("/directories")
	{
		// 计算目录磁盘占比和大小
		userDirGroup.GET("/calculate-usage", middleware.WithOperation("计算目录磁盘占比和大小"), ctrl.GetDiskUsage)
		// 列出 /home 目录下的所有用户目录
		userDirGroup.GET("/list-home", middleware.WithOperation("列出 /home 目录下的所有用户目录"), ctrl.ListHomeDirectories)
	}
}
