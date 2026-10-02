package processing

import (
	"net/http"

	"pal-next-gallery-server/app/auth"

	"github.com/gin-gonic/gin"
)

func (service *Service) Status(context *gin.Context) {
	identity, ok := context.Get(auth.UserContextKey)
	user, valid := identity.(auth.User)
	if !ok || !valid {
		context.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	job, err := service.repository.GetProcessingStatus(context.Request.Context(), context.Param("id"), user.ID)
	if err != nil {
		context.JSON(http.StatusNotFound, gin.H{"error": "processing job not found"})
		return
	}
	context.JSON(http.StatusOK, job)
}

func (service *Service) ListJobs(context *gin.Context) {
	jobs, err := service.repository.ListProcessingJobs(context.Request.Context())
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	context.JSON(http.StatusOK, jobs)
}

func (service *Service) RetryJob(context *gin.Context) {
	if err := service.repository.RetryProcessingJob(context.Request.Context(), context.Param("id")); err != nil {
		context.JSON(http.StatusNotFound, gin.H{"error": "processing job not found"})
		return
	}
	context.Status(http.StatusNoContent)
}
