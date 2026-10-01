package uploader

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func SetupRouter(router *gin.Engine, service IUploaderService) (*gin.Engine, error) {
	if router == nil {
		return nil, fmt.Errorf("router is required")
	}
	if err := RegisterRoutes(router, service); err != nil {
		return nil, err
	}
	return router, nil
}

func RegisterRoutes(router gin.IRoutes, service IUploaderService) error {
	if router == nil || service == nil {
		return fmt.Errorf("router and uploader service are required")
	}
	router.POST("/media/upload/check", service.HandleCheckDuplicate)
	router.POST("/media/upload", service.HandleCreateUpload)
	router.PUT("/media/upload/:id/parts/:part", service.HandleUploadPart)
	router.POST("/media/upload/:id/complete", service.HandleCompleteUpload)
	router.GET("/media/upload/:id", service.HandleGetUploadStatus)
	router.DELETE("/media/upload/:id", service.HandleDeleteUpload)
	return nil
}
