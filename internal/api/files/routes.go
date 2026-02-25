package files

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册文件路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 文件管理接口 /api/files

	r.Use(middleware.Auth())
	r.Use(middleware.UserOperationLogs(ctrl.userLogService))
	{
		// 1. 系统根目录管理 (/)
		systemGroup := r.Group("/system")
		systemGroup.Use(middleware.RequirePermission(ctrl.authService))
		{
			systemGroup.GET("/list", middleware.WithOperation("获取文件列表"), ctrl.GetFileList)
			systemGroup.DELETE("/delete", middleware.WithOperation("删除文件"), ctrl.DeleteFile)
			systemGroup.POST("/upload", middleware.WithOperation("上传文件"), ctrl.UploadFile)
			systemGroup.GET("/download", middleware.WithOperation("下载文件"), ctrl.DownloadFile)
			systemGroup.POST("/unzip", middleware.WithOperation("解压文件"), ctrl.UnzipFile)
			systemGroup.GET("/size", middleware.WithOperation("计算文件大小"), ctrl.CalculateSize)
		}

		// 2. 用户家目录管理 (/home/{username})
		userGroup := r.Group("/user")
		userGroup.Use(middleware.RequirePermission(ctrl.authService))
		{
			userGroup.GET("/list", middleware.WithOperation("获取文件列表"), ctrl.GetFileList)
			userGroup.DELETE("/delete", middleware.WithOperation("删除文件"), ctrl.DeleteFile)
			userGroup.POST("/upload", middleware.WithOperation("上传文件"), ctrl.UploadFile)
			userGroup.GET("/download", middleware.WithOperation("下载文件"), ctrl.DownloadFile)
			userGroup.POST("/unzip", middleware.WithOperation("解压文件"), ctrl.UnzipFile)
			userGroup.GET("/size", middleware.WithOperation("计算文件大小"), ctrl.CalculateSize)
		}

	}

	// 用户目录接口 /api/user/directories
	userDirGroup := r.Group("/user/directories")
	userDirGroup.Use(middleware.Auth())
	{
		// 计算目录磁盘占比和大小
		userDirGroup.GET("/calculate-usage", ctrl.GetDiskUsage)
	}
}
