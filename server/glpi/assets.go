package glpi

import (
	"context"
	"strconv"
)

// AssetItemTypes maps user-facing asset type names to GLPI item types.
// Item types listed in assetHasUserLink support filtering by assigned user.
var AssetItemTypes = map[string]string{
	"computers": "Computer",
	"printers":  "Printer",
	"monitors":  "Monitor",
	"network":   "NetworkEquipment",
	"software":  "Software",
	"licenses":  "SoftwareLicense",
}

var assetHasUserLink = map[string]bool{
	"Computer":         true,
	"Printer":          true,
	"Monitor":          true,
	"NetworkEquipment": true,
}

var assetHasSerial = map[string]bool{
	"Computer":         true,
	"Printer":          true,
	"Monitor":          true,
	"NetworkEquipment": true,
}

// AssetFilter describes which assets to look up.
type AssetFilter struct {
	// ItemType is the GLPI item type, e.g. "Computer".
	ItemType string
	// GLPIUserID, when > 0 and the item type supports it, restricts results to
	// assets assigned to that user.
	GLPIUserID int
	// NameQuery, when set, restricts results to assets whose name contains it.
	NameQuery string
	Limit     int
}

// AssetSummary is a compact asset row returned by the search engine.
type AssetSummary struct {
	ID     int
	Name   string
	Serial string
}

// SupportsUserFilter reports whether the given GLPI item type can be filtered
// by assigned user.
func SupportsUserFilter(itemType string) bool {
	return assetHasUserLink[itemType]
}

// SearchAssets queries GLPI assets of the given item type.
func (c *Client) SearchAssets(ctx context.Context, filter AssetFilter) ([]AssetSummary, int, error) {
	if filter.ItemType == "" {
		return nil, 0, &ConfigError{Message: "asset item type is empty"}
	}

	var criteria []searchCriterion

	if filter.GLPIUserID > 0 && assetHasUserLink[filter.ItemType] {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(assetFieldUser),
			SearchType: "equals",
			Value:      strconv.Itoa(filter.GLPIUserID),
		})
	}
	if filter.NameQuery != "" {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(fieldName),
			SearchType: "contains",
			Value:      filter.NameQuery,
		})
	}
	if len(criteria) == 0 {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(fieldID),
			SearchType: "morethan",
			Value:      "0",
		})
	}

	display := []int{fieldID, fieldName}
	includeSerial := assetHasSerial[filter.ItemType]
	if includeSerial {
		display = append(display, assetFieldSerial)
	}

	result, err := c.runSearch(ctx, searchQuery{
		ItemType:     filter.ItemType,
		Criteria:     criteria,
		ForceDisplay: display,
		Limit:        filter.Limit,
	})
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]AssetSummary, 0, len(result.Data))
	for _, row := range result.Data {
		summary := AssetSummary{
			ID:   asInt(row[strconv.Itoa(fieldID)]),
			Name: asString(row[strconv.Itoa(fieldName)]),
		}
		if includeSerial {
			summary.Serial = asString(row[strconv.Itoa(assetFieldSerial)])
		}
		summaries = append(summaries, summary)
	}
	return summaries, result.TotalCount, nil
}
