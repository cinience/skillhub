package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/cinience/skillhub/pkg/search"
)

type SearchHandler struct {
	searchClient *search.Client
}

func NewSearchHandler(sc *search.Client) *SearchHandler {
	return &SearchHandler{searchClient: sc}
}

// Search handles GET /api/v1/search
func (h *SearchHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	var sortFields []string
	if s := c.Query("sort"); s != "" {
		sortFields = []string{s}
	}

	filters := "moderationStatus = approved AND isDeleted = false"

	result, err := h.searchClient.Search(c.Request.Context(), query, limit, offset, sortFields, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
