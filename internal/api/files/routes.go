package files

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册文件路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 文件管理接口 /api/files

	r.Use(middleware.Auth())
	r.Use(middleware.RequirePermission(ctrl.authService))
	{
		// 文件管理 (通过 :mode 参数区分 system/user)
		fileGroup := r.Group("/:mode")
		{
			fileGroup.GET("/list", ctrl.GetFileList)
			fileGroup.DELETE("/delete", ctrl.DeleteFile)
			fileGroup.POST("/upload", ctrl.UploadFile)
			fileGroup.GET("/download", ctrl.DownloadFile)
			fileGroup.POST("/unzip", ctrl.UnzipFile)
			fileGroup.GET("/size", ctrl.CalculateSize)
		}

	}

	// 用户目录接口 /api/user/directories
	userDirGroup := r.Group("/directories")
	userDirGroup.Use(middleware.Auth())
	userDirGroup.Use(middleware.RequirePermission(ctrl.authService))
	{
		// 计算目录磁盘占比和大小
		userDirGroup.GET("/calculate-usage", ctrl.GetDiskUsage)
		// 列出 /home 目录下的所有用户目录
		userDirGroup.GET("/list-home", ctrl.ListHomeDirectories)
	}
}
