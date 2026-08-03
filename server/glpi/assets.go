package glpi

import (
	"context"
	"net/http"
	"net/url"
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
	Page      int // 1-based page; 0 or 1 = first page
}

// AssetSummary is a compact asset row returned by the search engine.
type AssetSummary struct {
	ID       int
	Name     string
	Serial   string
	ItemType string
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
		Page:         filter.Page,
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

// AssetDetail is a normalized single-asset view with human-readable dropdown
// labels (manufacturer, model, location, users) resolved by GLPI.
type AssetDetail struct {
	ID           int    `json:"id"`
	ItemType     string `json:"itemtype"`
	Name         string `json:"name"`
	Serial       string `json:"serial"`
	OtherSerial  string `json:"otherserial"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Location     string `json:"location"`
	User         string `json:"user"`
	TechUser     string `json:"tech_user"`
	State        string `json:"state"`
	WarrantyDate string `json:"warranty_date"`
	Notes        string `json:"notes"`
}

// GetAsset retrieves a single asset of the given GLPI item type with its
// dropdown relations expanded to human-readable labels.
func (c *Client) GetAsset(ctx context.Context, itemType string, id int) (*AssetDetail, error) {
	values := url.Values{}
	values.Set("expand_dropdowns", "true")

	var raw map[string]interface{}
	if err := c.doRequest(ctx, http.MethodGet, "/apirest.php/"+itemType+"/"+strconv.Itoa(id), values, nil, &raw); err != nil {
		return nil, err
	}

	return &AssetDetail{
		ID:           asInt(raw["id"]),
		ItemType:     itemType,
		Name:         asString(raw["name"]),
		Serial:       asString(raw["serial"]),
		OtherSerial:  asString(raw["otherserial"]),
		Manufacturer: dropdownName(raw["manufacturers_id"]),
		Model:        dropdownName(raw["models_id"]),
		Location:     dropdownName(raw["locations_id"]),
		User:         dropdownName(raw["users_id"]),
		TechUser:     dropdownName(raw["users_id_tech"]),
		State:        dropdownName(raw["states_id"]),
		WarrantyDate: asString(raw["warranty_date"]),
		Notes:        asString(raw["notes"]),
	}, nil
}
