package handlers

import (
	"github.com/gin-gonic/gin"
)

// setupTestRouter creates a test router for handler tests
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}
