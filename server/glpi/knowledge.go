package glpi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// KnowledgeSummary is a compact knowledge base article row.
type KnowledgeSummary struct {
	ID      int
	Subject string
}

// knowbaseFieldCategory is the KnowbaseItem search option for its category.
const knowbaseFieldCategory = 3

// KnowbaseCategorySummary is a compact knowledge base category row.
type KnowbaseCategorySummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SearchKnowledgeBaseCategories lists GLPI knowledge base categories.
func (c *Client) SearchKnowledgeBaseCategories(ctx context.Context, limit int) ([]KnowbaseCategorySummary, int, error) {
	if limit <= 0 {
		limit = 50
	}
	result, err := c.runSearch(ctx, searchQuery{
		ItemType:     "KnowbaseItemCategory",
		ForceDisplay: []int{fieldID, fieldName},
		Sort:         fieldName,
		Order:        "ASC",
		Limit:        limit,
	})
	if err != nil {
		return nil, 0, err
	}
	categories := make([]KnowbaseCategorySummary, 0, len(result.Data))
	for _, row := range result.Data {
		categories = append(categories, KnowbaseCategorySummary{
			ID:   asInt(row[strconv.Itoa(fieldID)]),
			Name: asString(row[strconv.Itoa(fieldName)]),
		})
	}
	return categories, result.TotalCount, nil
}

// SearchKnowledge searches the GLPI knowledge base by subject, optionally
// restricted to a single category.
func (c *Client) SearchKnowledge(ctx context.Context, query string, categoryID, limit, page int) ([]KnowledgeSummary, int, error) {
	if query == "" {
		return nil, 0, &ConfigError{Message: "knowledge base query is empty"}
	}

	criteria := []searchCriterion{
		{
			Field:      strconv.Itoa(fieldName),
			SearchType: "contains",
			Value:      query,
		},
	}
	if categoryID > 0 {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(knowbaseFieldCategory),
			SearchType: "equals",
			Value:      strconv.Itoa(categoryID),
		})
	}

	result, err := c.runSearch(ctx, searchQuery{
		ItemType:     "KnowbaseItem",
		Criteria:     criteria,
		ForceDisplay: []int{fieldID, fieldName},
		Limit:        limit,
		Page:         page,
	})
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]KnowledgeSummary, 0, len(result.Data))
	for _, row := range result.Data {
		summaries = append(summaries, KnowledgeSummary{
			ID:      asInt(row[strconv.Itoa(fieldID)]),
			Subject: asString(row[strconv.Itoa(fieldName)]),
		})
	}
	return summaries, result.TotalCount, nil
}

// KnowledgeArticle is a full knowledge base article.
type KnowledgeArticle struct {
	ID       int    `json:"id"`
	Subject  string `json:"subject"`
	Content  string `json:"content"`
	Category string `json:"category"`
	Date     string `json:"date"`
	DateMod  string `json:"date_mod"`
}

// GetKnowbaseItem retrieves a single knowledge base article by ID. The article
// body is GLPI rich-text (HTML) and is returned as-is; callers sanitize it
// before rendering.
func (c *Client) GetKnowbaseItem(ctx context.Context, id int) (*KnowledgeArticle, error) {
	values := url.Values{}
	values.Set("expand_dropdowns", "true")

	var raw map[string]interface{}
	if err := c.doRequest(ctx, http.MethodGet, "/apirest.php/KnowbaseItem/"+strconv.Itoa(id), values, nil, &raw); err != nil {
		return nil, err
	}

	return &KnowledgeArticle{
		ID:       asInt(raw["id"]),
		Subject:  asString(firstKnown(raw, "name", "subject")),
		Content:  asString(raw["answer"]),
		Category: dropdownName(raw["knowbaseitemcategories_id"]),
		Date:     asString(firstKnown(raw, "date", "date_creation")),
		DateMod:  asString(raw["date_mod"]),
	}, nil
}
