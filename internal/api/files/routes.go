package files

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册文件路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 文件管理接口 /api/files

	r.Use(middleware.Auth())
	{
		// 1. 系统根目录管理 (/)
		systemGroup := r.Group("/system")
		systemGroup.Use(middleware.RequirePermission(ctrl.authService))
		{
			systemGroup.GET("/list", ctrl.GetFileList)
			systemGroup.DELETE("/delete", ctrl.DeleteFile)
			systemGroup.POST("/upload", ctrl.UploadFile)
			systemGroup.GET("/download", ctrl.DownloadFile)
			systemGroup.POST("/unzip", ctrl.UnzipFile)
			systemGroup.GET("/size", ctrl.CalculateSize)
		}

		// 2. 用户家目录管理 (/home/{username})
		userGroup := r.Group("/user")
		userGroup.Use(middleware.RequirePermission(ctrl.authService))
		{
			userGroup.GET("/list", ctrl.GetFileList)
			userGroup.DELETE("/delete", ctrl.DeleteFile)
			userGroup.POST("/upload", ctrl.UploadFile)
			userGroup.GET("/download", ctrl.DownloadFile)
			userGroup.POST("/unzip", ctrl.UnzipFile)
			userGroup.GET("/size", ctrl.CalculateSize)
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
