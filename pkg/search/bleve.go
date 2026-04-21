package search

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	bleveSearch "github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/searcher"
	"github.com/cinience/skillhub/pkg/config"

	// Ensure searcher init runs (needed for DisjunctionMaxClauseCount).
	_ "github.com/blevesearch/bleve/v2/search/searcher"
)

// Suppress unused import warning.
var _ = searcher.DisjunctionMaxClauseCount

type Client struct {
	index bleve.Index
}

func New(cfg config.SearchConfig) (*Client, error) {
	indexPath := cfg.IndexPath
	if indexPath == "" {
		indexPath = "./data/skills.bleve"
	}

	var idx bleve.Index
	var err error

	if _, statErr := os.Stat(indexPath); os.IsNotExist(statErr) {
		idx, err = bleve.New(indexPath, buildMapping())
	} else {
		idx, err = bleve.Open(indexPath)
	}
	if err != nil {
		return nil, fmt.Errorf("open bleve index: %w", err)
	}

	return &Client{index: idx}, nil
}

func buildMapping() mapping.IndexMapping {
	skillMapping := bleve.NewDocumentMapping()

	textField := bleve.NewTextFieldMapping()
	keywordField := bleve.NewKeywordFieldMapping()
	numericField := bleve.NewNumericFieldMapping()
	boolField := bleve.NewBooleanFieldMapping()

	// Searchable text fields
	skillMapping.AddFieldMappingsAt("slug", textField)
	skillMapping.AddFieldMappingsAt("displayName", textField)
	skillMapping.AddFieldMappingsAt("summary", textField)
	skillMapping.AddFieldMappingsAt("skillMdContent", textField)
	skillMapping.AddFieldMappingsAt("tags", textField)
	skillMapping.AddFieldMappingsAt("ownerHandle", textField)

	// Filterable keyword fields
	skillMapping.AddFieldMappingsAt("category", keywordField)
	skillMapping.AddFieldMappingsAt("visibility", keywordField)
	skillMapping.AddFieldMappingsAt("moderationStatus", keywordField)
	skillMapping.AddFieldMappingsAt("ownerHandleExact", keywordField)

	// Boolean filter fields
	skillMapping.AddFieldMappingsAt("isSuspicious", boolField)
	skillMapping.AddFieldMappingsAt("isDeleted", boolField)

	// Sortable numeric fields
	skillMapping.AddFieldMappingsAt("downloads", numericField)
	skillMapping.AddFieldMappingsAt("stars", numericField)
	skillMapping.AddFieldMappingsAt("updatedAt", numericField)
	skillMapping.AddFieldMappingsAt("createdAt", numericField)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = skillMapping
	return indexMapping
}

type SkillDocument struct {
	ID               string   `json:"id"`
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"displayName"`
	Summary          string   `json:"summary"`
	SkillMdContent   string   `json:"skillMdContent"`
	Category         string   `json:"category"`
	Tags             []string `json:"tags"`
	OwnerHandle      string   `json:"ownerHandle"`
	OwnerHandleExact string   `json:"ownerHandleExact"`
	Visibility       string   `json:"visibility"`
	ModerationStatus string   `json:"moderationStatus"`
	IsSuspicious     bool     `json:"isSuspicious"`
	IsDeleted        bool     `json:"isDeleted"`
	Downloads        int64    `json:"downloads"`
	Stars            int      `json:"stars"`
	UpdatedAt        int64    `json:"updatedAt"`
	CreatedAt        int64    `json:"createdAt"`
}

// IndexSkill adds or updates a skill in the search index.
func (c *Client) IndexSkill(ctx context.Context, doc *SkillDocument) error {
	doc.OwnerHandleExact = doc.OwnerHandle
	return c.index.Index(doc.ID, doc)
}

// DeleteSkill removes a skill from the search index.
func (c *Client) DeleteSkill(ctx context.Context, id string) error {
	return c.index.Delete(id)
}

type SearchResult struct {
	Hits             []map[string]interface{} `json:"hits"`
	Query            string                   `json:"query"`
	ProcessingTimeMs int64                    `json:"processingTimeMs"`
	EstimatedTotal   int64                    `json:"estimatedTotalHits"`
}

// Search performs a full-text search with optional sorting and filtering.
func (c *Client) Search(ctx context.Context, query string, limit, offset int, sort []string, filters string) (*SearchResult, error) {
	q := bleve.NewQueryStringQuery(query)
	searchReq := bleve.NewSearchRequestOptions(q, limit, offset, false)
	searchReq.Fields = []string{"*"}

	// Apply sort
	if len(sort) > 0 {
		var sortOrder bleveSearch.SortOrder
		for _, s := range sort {
			field := s
			desc := false
			if strings.HasSuffix(s, ":desc") {
				field = strings.TrimSuffix(s, ":desc")
				desc = true
			} else if strings.HasSuffix(s, ":asc") {
				field = strings.TrimSuffix(s, ":asc")
			} else {
				// Default: desc for numeric fields
				desc = true
			}
			sf := &bleveSearch.SortField{Field: field, Desc: desc, Type: bleveSearch.SortFieldAsNumber}
			sortOrder = append(sortOrder, sf)
		}
		searchReq.SortByCustom(sortOrder)
	}

	// Apply filters by wrapping the query in a conjunction with term queries
	if filters != "" {
		finalQuery := bleve.NewConjunctionQuery(q)
		parts := strings.Split(filters, " AND ")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, " = ", 2)
			if len(kv) != 2 {
				continue
			}
			field := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch value {
			case "true":
				bq := bleve.NewBoolFieldQuery(true)
				bq.SetField(field)
				finalQuery.AddQuery(bq)
			case "false":
				bq := bleve.NewBoolFieldQuery(false)
				bq.SetField(field)
				finalQuery.AddQuery(bq)
			default:
				tq := bleve.NewTermQuery(value)
				tq.SetField(field)
				finalQuery.AddQuery(tq)
			}
		}
		searchReq = bleve.NewSearchRequestOptions(finalQuery, limit, offset, false)
		searchReq.Fields = []string{"*"}
		// Re-apply sort order (was set on original request which is now replaced)
		if len(sort) > 0 {
			var sortOrder bleveSearch.SortOrder
			for _, s := range sort {
				field := s
				desc := false
				if strings.HasSuffix(s, ":desc") {
					field = strings.TrimSuffix(s, ":desc")
					desc = true
				} else if strings.HasSuffix(s, ":asc") {
					field = strings.TrimSuffix(s, ":asc")
				} else {
					desc = true
				}
				sf := &bleveSearch.SortField{Field: field, Desc: desc, Type: bleveSearch.SortFieldAsNumber}
				sortOrder = append(sortOrder, sf)
			}
			searchReq.SortByCustom(sortOrder)
		}
	}

	result, err := c.index.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("bleve search: %w", err)
	}

	hits := make([]map[string]interface{}, 0, len(result.Hits))
	for _, hit := range result.Hits {
		m := make(map[string]interface{})
		m["id"] = hit.ID
		for k, v := range hit.Fields {
			m[k] = v
		}
		hits = append(hits, m)
	}

	return &SearchResult{
		Hits:             hits,
		Query:            query,
		ProcessingTimeMs: result.Took.Milliseconds(),
		EstimatedTotal:   int64(result.Total),
	}, nil
}

// EnsureIndex is a no-op for Bleve (index is created on New).
func (c *Client) EnsureIndex(ctx context.Context) error {
	return nil
}

// Healthy always returns true for the embedded Bleve index.
func (c *Client) Healthy(ctx context.Context) bool {
	return true
}

// Close closes the Bleve index.
func (c *Client) Close() error {
	return c.index.Close()
}
