package handlers

import (
	"encoding/json"
	"net/http"

	"backend/application/services"
	"backend/pkg/auth"
	"backend/pkg/errors"
	"go.uber.org/zap"
)

// CommunityHandler handles community detection HTTP requests.
type CommunityHandler struct {
	communityService *services.CommunityDetectionService
	logger           *zap.Logger
	errorHandler     *errors.ErrorHandler
}

// NewCommunityHandler creates a new community handler.
func NewCommunityHandler(
	communityService *services.CommunityDetectionService,
	logger *zap.Logger,
	errorHandler *errors.ErrorHandler,
) *CommunityHandler {
	return &CommunityHandler{
		communityService: communityService,
		logger:           logger,
		errorHandler:     errorHandler,
	}
}

// Recompute handles POST /communities/recompute — runs Leiden detection.
func (h *CommunityHandler) Recompute(w http.ResponseWriter, r *http.Request) {
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	result, err := h.communityService.DetectCommunities(r.Context(), userCtx.UserID)
	if err != nil {
		h.logger.Error("Community detection failed",
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Community detection failed").WithCause(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}
