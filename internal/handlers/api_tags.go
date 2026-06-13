package handlers

import (
	"net/http"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// Tag API response with additional fields
type TagDetailResponse struct {
	ID         uuid.UUID           `json:"id"`
	LedgerID   uuid.UUID           `json:"ledger_id"`
	ParentID   *uuid.UUID          `json:"parent_id,omitempty"`
	Name       string              `json:"name"`
	Color      string              `json:"color"`
	UsageCount int                 `json:"usage_count"`
	Children   []TagDetailResponse `json:"children,omitempty"`
	CreatedAt  string              `json:"created_at"`
	UpdatedAt  string              `json:"updated_at"`
}

func tagToDetailResponse(tag *models.Tag, usageCount int) TagDetailResponse {
	resp := TagDetailResponse{
		ID:         tag.ID,
		LedgerID:   tag.LedgerID,
		ParentID:   tag.ParentID,
		Name:       tag.Name,
		Color:      tag.Color,
		UsageCount: usageCount,
		CreatedAt:  tag.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  tag.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if tag.Children != nil {
		resp.Children = make([]TagDetailResponse, len(tag.Children))
		for i, child := range tag.Children {
			resp.Children[i] = tagToDetailResponse(child, 0) // Children don't have usage counts in hierarchy
		}
	}

	return resp
}

func respondTagDetail(w http.ResponseWriter, r *http.Request, h *APIHandlers, tag *models.Tag) {
	usageCount, err := h.tags.GetTagUsageCount(r.Context(), tag.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, tagToDetailResponse(tag, usageCount))
}

// APITagsList returns all tags for the current ledger
func (h *APIHandlers) APITagsList(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	// Check if hierarchical view is requested
	hierarchy := r.URL.Query().Get("hierarchy") == "true"

	var (
		tags []*models.Tag
		err  error
	)
	if hierarchy {
		tags, err = h.tags.GetHierarchy(r.Context(), ledger.ID)
	} else {
		tags, err = h.tags.GetByLedgerID(r.Context(), ledger.ID)
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get usage counts
	usageCounts, err := h.tags.GetTagUsageCounts(r.Context(), ledger.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format
	result := make([]TagDetailResponse, len(tags))
	for i, tag := range tags {
		result[i] = tagToDetailResponse(tag, usageCounts[tag.ID])
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

type createTagRequest struct {
	Name     string     `json:"name"`
	Color    string     `json:"color,omitempty"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// APITagsCreate creates a new tag
func (h *APIHandlers) APITagsCreate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	var req createTagRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Validate parent if provided
	if req.ParentID != nil {
		parent, err := h.tags.GetByID(r.Context(), *req.ParentID)
		if err != nil || parent.LedgerID != ledger.ID {
			respondError(w, http.StatusBadRequest, "invalid parent_id")
			return
		}
	}

	tag := &models.Tag{
		LedgerID: ledger.ID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Color:    req.Color,
	}

	if tag.Color == "" {
		tag.Color = "#6366f1" // Default indigo
	}

	if err := h.tags.Create(r.Context(), tag); err != nil {
		// Check for unique constraint violation
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, tagToDetailResponse(tag, 0))
}

// APITagsGet returns a single tag by ID
func (h *APIHandlers) APITagsGet(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	tagID, ok := mustAPIParamUUID(w, r, "id", "tag ID")
	if !ok {
		return
	}

	tag, ok := h.getOwnedTag(w, r, tagID, ledger.ID)
	if !ok {
		return
	}

	respondTagDetail(w, r, h, tag)
}

type updateTagRequest struct {
	Name     *string    `json:"name,omitempty"`
	Color    *string    `json:"color,omitempty"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// APITagsUpdate updates a tag
func (h *APIHandlers) APITagsUpdate(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	tagID, ok := mustAPIParamUUID(w, r, "id", "tag ID")
	if !ok {
		return
	}

	tag, ok := h.getOwnedTag(w, r, tagID, ledger.ID)
	if !ok {
		return
	}

	var req updateTagRequest
	if err := parseJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name != nil {
		tag.Name = *req.Name
	}
	if req.Color != nil {
		tag.Color = *req.Color
	}
	if req.ParentID != nil {
		// Validate parent
		if *req.ParentID != uuid.Nil {
			parent, err := h.tags.GetByID(r.Context(), *req.ParentID)
			if err != nil || parent.LedgerID != ledger.ID {
				respondError(w, http.StatusBadRequest, "invalid parent_id")
				return
			}
			// Prevent circular reference
			if *req.ParentID == tagID {
				respondError(w, http.StatusBadRequest, "tag cannot be its own parent")
				return
			}
		}
		tag.ParentID = req.ParentID
	}

	if err := h.tags.Update(r.Context(), tag); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondTagDetail(w, r, h, tag)
}

// APITagsDelete deletes a tag
func (h *APIHandlers) APITagsDelete(w http.ResponseWriter, r *http.Request) {
	ledger, ok := h.requireAPILedger(w, r)
	if !ok {
		return
	}

	tagID, ok := mustAPIParamUUID(w, r, "id", "tag ID")
	if !ok {
		return
	}

	tag, ok := h.getOwnedTag(w, r, tagID, ledger.ID)
	if !ok {
		return
	}

	if err := h.tags.Delete(r.Context(), tag.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondDeleted(w)
}

func (h *APIHandlers) getOwnedTag(w http.ResponseWriter, r *http.Request, id, ledgerID uuid.UUID) (*models.Tag, bool) {
	tag, err := h.tags.GetByID(r.Context(), id)
	if err != nil || tag.LedgerID != ledgerID {
		respondError(w, http.StatusNotFound, "tag not found")
		return nil, false
	}
	return tag, true
}
