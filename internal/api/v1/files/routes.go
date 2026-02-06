package files

import (
	"cloudque/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册文件路由
func (ctrl *Controller) RegisterRoutes(r *gin.RouterGroup) {
	// 文件管理接口 /api/v1/files
	filesGroup := r.Group("/files")
	filesGroup.Use(middleware.Auth())
	{
		filesGroup.GET("/list", ctrl.GetFileList)        // 获取用户文件列表
		filesGroup.DELETE("/delete", ctrl.DeleteFile)    // 删除文件或目录
		filesGroup.POST("/upload_file", ctrl.UploadFile) // 上传文件
		filesGroup.GET("/download", ctrl.DownloadFile)   // 文件下载
		filesGroup.POST("/unzip", ctrl.UnzipFile)        // 解压文件
		filesGroup.POST("/chmod", ctrl.Chmod)            // 修改权限
		filesGroup.GET("/size", ctrl.CalculateSize)      // 计算大小
	}

	// 用户目录接口 /api/v1/user/directories
	userDirGroup := r.Group("/user/directories")
	userDirGroup.Use(middleware.Auth())
	{
		// 计算目录磁盘占比和大小
		userDirGroup.GET("/calculate-usage", ctrl.GetDiskUsage)
	}
}
