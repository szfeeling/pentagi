package services

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"pentagi/pkg/wireless"

	"github.com/gin-gonic/gin"
)

type WirelessService struct{ Service *wireless.Service }

// @Summary Submit wireless node heartbeat, observations and job receipts
// @Tags Wireless
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer node enrollment token"
// @Param report body wireless.Report true "Node report"
// @Success 200 {object} wireless.Dispatch
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /wireless/node/report [post]
func (s *WirelessService) Report(c *gin.Context) {
	if s.Service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wireless service unavailable"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var report wireless.Report
	if err := decoder.Decode(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wireless report"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unexpected report data"})
		return
	}
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "node token required"})
		return
	}
	dispatch, err := s.Service.Report(c.Request.Context(), strings.TrimPrefix(header, "Bearer "), report)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, wireless.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		if errors.Is(err, wireless.ErrConflict) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dispatch)
}
