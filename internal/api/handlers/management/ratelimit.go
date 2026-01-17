package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// GetRateLimits returns the current rate limit rules.
func (h *Handler) GetRateLimits(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.JSON(http.StatusOK, h.cfg.RateLimit)
}

// PutRateLimits updates the rate limit rules.
func (h *Handler) PutRateLimits(c *gin.Context) {
	var body config.RateLimitConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	h.mu.Lock()
	h.cfg.RateLimit = body
	h.mu.Unlock()

	if h.OnConfigUpdated != nil {
		h.OnConfigUpdated()
	}

	h.persist(c)
}
