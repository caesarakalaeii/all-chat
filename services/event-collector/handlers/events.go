package handlers

import (
	"fmt"
	"net/http"

	"github.com/caesar/all-chat/services/event-collector/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EventsHandler handles event-related API endpoints
type EventsHandler struct {
	eventRepo   *repository.EventRepository
	sessionRepo *repository.SessionRepository
	logger      *zap.Logger
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(
	eventRepo *repository.EventRepository,
	sessionRepo *repository.SessionRepository,
	logger *zap.Logger,
) *EventsHandler {
	return &EventsHandler{
		eventRepo:   eventRepo,
		sessionRepo: sessionRepo,
		logger:      logger,
	}
}

// GetEventsBySession handles GET /api/v1/sessions/:id/events
func (h *EventsHandler) GetEventsBySession(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	// Optional filter by event type
	eventType := c.Query("type")

	var events interface{}
	if eventType != "" {
		events, err = h.eventRepo.GetEventsBySessionAndType(c.Request.Context(), sessionID, eventType)
	} else {
		events, err = h.eventRepo.GetEventsBySession(c.Request.Context(), sessionID)
	}

	if err != nil {
		h.logger.Error("Failed to get events", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"events":     events,
	})
}

// GetSessionStats handles GET /api/v1/sessions/:id/stats
func (h *EventsHandler) GetSessionStats(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	session, err := h.sessionRepo.GetSessionByID(c.Request.Context(), sessionID)
	if err != nil {
		h.logger.Error("Failed to get session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve session"})
		return
	}

	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"stats":      session.Stats,
		"started_at": session.StartedAt,
		"ended_at":   session.EndedAt,
		"status":     session.Status,
	})
}

// GetUserSessions handles GET /api/v1/users/:id/sessions
func (h *EventsHandler) GetUserSessions(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Optional limit parameter
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	sessions, err := h.sessionRepo.GetSessionsByUser(c.Request.Context(), userID, limit)
	if err != nil {
		h.logger.Error("Failed to get sessions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// GetActiveSession handles GET /api/v1/users/:id/sessions/active
func (h *EventsHandler) GetActiveSession(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	session, err := h.sessionRepo.GetActiveSession(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get active session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve session"})
		return
	}

	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active session found"})
		return
	}

	c.JSON(http.StatusOK, session)
}
