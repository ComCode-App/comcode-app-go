package handler

import (
	"encoding/json"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/opengraph"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	ogClient *opengraph.Client
}

func New(ogClient *opengraph.Client) *Handler {
	return &Handler{ogClient: ogClient}
}

type LinkMetadataRequest struct {
	URL string `json:"url"`
}

func (h *Handler) GetLinkMetadata(w http.ResponseWriter, r *http.Request) {
	var req LinkMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	if req.URL == "" {
		response.BadRequest(w, r, response.CodeValidationError, "url is required")
		return
	}

	meta, err := h.ogClient.Fetch(r.Context(), req.URL)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, meta)
}
