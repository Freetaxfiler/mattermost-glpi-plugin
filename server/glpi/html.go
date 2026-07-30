package glpi

import (
	"html"
	"strings"
)

// StripHTML converts GLPI rich-text (HTML) content to plain text suitable for
// display inside a Mattermost message. It is intentionally simple: tags are
// removed, common block-level tags become line breaks, and entities are decoded.
func StripHTML(input string) string {
	if input == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
		"</li>", "\n",
	)
	normalized := replacer.Replace(input)

	var builder strings.Builder
	builder.Grow(len(normalized))
	inTag := false
	for _, r := range normalized {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			builder.WriteRune(r)
		}
	}

	text := html.UnescapeString(builder.String())

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}
