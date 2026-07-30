package glpi

import (
	"context"
	"strconv"
)

// KnowledgeSummary is a compact knowledge base article row.
type KnowledgeSummary struct {
	ID      int
	Subject string
}

// SearchKnowledge searches the GLPI knowledge base by subject.
func (c *Client) SearchKnowledge(ctx context.Context, query string, limit int) ([]KnowledgeSummary, int, error) {
	if query == "" {
		return nil, 0, &ConfigError{Message: "knowledge base query is empty"}
	}

	result, err := c.runSearch(ctx, searchQuery{
		ItemType: "KnowbaseItem",
		Criteria: []searchCriterion{
			{
				Field:      strconv.Itoa(fieldName),
				SearchType: "contains",
				Value:      query,
			},
		},
		ForceDisplay: []int{fieldID, fieldName},
		Limit:        limit,
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
