package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

func assetTypeNames() string {
	names := make([]string, 0, len(glpi.AssetItemTypes))
	for name := range glpi.AssetItemTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func executeAssets(ctx context.Context, p PluginExecutor, args *model.CommandArgs, rest []string) (*model.CommandResponse, *model.AppError) {
	if !checkAndRecordUserRate(ctx) {
		return responseText("You're requesting assets too frequently. Please wait a moment and try again."), nil
	}
	usage := fmt.Sprintf("Usage: `/glpi assets [type] [search]`\nAsset types: %s\nWith no search text, hardware types show assets assigned to you.", assetTypeNames())

	typeName := "computers"
	if len(rest) > 0 {
		typeName = strings.ToLower(rest[0])
	}
	itemType, ok := glpi.AssetItemTypes[typeName]
	if !ok {
		return responseText(fmt.Sprintf("`%s` is not a known asset type.\n\n%s", typeName, usage)), nil
	}

	nameQuery := ""
	if len(rest) > 1 {
		nameQuery = strings.TrimSpace(strings.Join(rest[1:], " "))
	}

	filter := glpi.AssetFilter{ItemType: itemType, NameQuery: nameQuery, Limit: listLimit}

	if nameQuery == "" {
		if glpi.SupportsUserFilter(itemType) {
			glpiUserID, errResp := resolveGLPIUser(p, args.UserId)
			if errResp != nil {
				return errResp, nil
			}
			filter.GLPIUserID = glpiUserID
		} else {
			return responseText(fmt.Sprintf("`%s` cannot be filtered by user. Add a search term, e.g. `/glpi assets %s office`.", typeName, typeName)), nil
		}
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	assets, total, err := client.SearchAssets(ctx, filter)
	if err != nil {
		return friendlyError("Searching assets", err), nil
	}

	title := fmt.Sprintf("%s assets", capitalize(typeName))
	if nameQuery != "" {
		title = fmt.Sprintf("%s matching `%s`", title, nameQuery)
	} else {
		title = fmt.Sprintf("Your %s", typeName)
	}

	if len(assets) == 0 {
		return responseText(fmt.Sprintf("### %s\nNo assets found.", title)), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s\n", title))
	b.WriteString("| ID | Name | Serial |\n")
	b.WriteString("|---:|---|---|\n")
	for _, asset := range assets {
		serial := asset.Serial
		if serial == "" {
			serial = "—"
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s |\n", asset.ID, escapePipes(asset.Name), escapePipes(serial)))
	}
	if total > len(assets) {
		b.WriteString(fmt.Sprintf("\nShowing %d of %d assets.", len(assets), total))
	}
	return responseText(b.String()), nil
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

func executeKnowledge(ctx context.Context, p PluginExecutor, rest []string) (*model.CommandResponse, *model.AppError) {
	if !checkAndRecordUserRate(ctx) {
		return responseText("You're searching the knowledge base too frequently. Please wait a moment and try again."), nil
	}

	query := strings.TrimSpace(strings.Join(rest, " "))
	if query == "" {
		return responseText("Usage: `/glpi kb <text>`"), nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	config := p.GetConfiguration()

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	articles, total, err := client.SearchKnowledge(ctx, query, listLimit)
	if err != nil {
		return friendlyError("Searching the knowledge base", err), nil
	}

	if len(articles) == 0 {
		return responseText(fmt.Sprintf("No knowledge base articles found for `%s`.", query)), nil
	}

	baseURL := strings.TrimRight(config.GLPIURL, "/")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### Knowledge base results for `%s`\n", query))
	for _, article := range articles {
		b.WriteString(fmt.Sprintf("- [%s](%s/front/knowbaseitem.form.php?id=%d)\n", escapePipes(article.Subject), baseURL, article.ID))
	}
	if total > len(articles) {
		b.WriteString(fmt.Sprintf("\nShowing %d of %d articles.", len(articles), total))
	}
	return responseText(b.String()), nil
}
