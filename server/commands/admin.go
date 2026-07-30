package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func executeAdmin(ctx context.Context, p PluginExecutor, config *ConfigView, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.IsSystemAdmin(args.UserId) {
		return responseText("Only system administrators can use `/glpi admin`."), nil
	}

	var b strings.Builder
	b.WriteString("### GLPI plugin diagnostics\n")
	b.WriteString("| Setting | Value |\n|---|---|\n")
	b.WriteString(fmt.Sprintf("| Plugin version | %s |\n", PluginVersion))
	b.WriteString(fmt.Sprintf("| GLPI URL | %s |\n", valueOrUnset(config.GLPIURL)))
	b.WriteString(fmt.Sprintf("| App token | %s |\n", secretState(config.AppToken)))
	b.WriteString(fmt.Sprintf("| User token | %s |\n", secretState(config.UserToken)))
	b.WriteString(fmt.Sprintf("| Default entity | %s |\n", valueOrUnset(config.DefaultEntity)))
	b.WriteString(fmt.Sprintf("| Default category | %s |\n", valueOrUnset(config.DefaultCategory)))
	b.WriteString(fmt.Sprintf("| Notification channel | %s |\n", valueOrUnset(config.NotificationChannelID)))
	b.WriteString(fmt.Sprintf("| Debug logging | %t |\n", config.EnableDebug))

	client := p.GetGLPIClient()
	if client == nil {
		b.WriteString("\n:warning: GLPI client is **not initialized**. Fix the configuration and save the plugin settings again.")
		return responseText(b.String()), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	result, err := client.HealthCheck(ctx)
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		b.WriteString(fmt.Sprintf("\n:red_circle: GLPI health check **failed** after %s: %v", elapsed, err))
	} else {
		b.WriteString(fmt.Sprintf("\n:large_green_circle: GLPI reachable in %s. Version: **%s**", elapsed, result.Version))
	}

	return responseText(b.String()), nil
}

func valueOrUnset(value string) string {
	if strings.TrimSpace(value) == "" {
		return "_not set_"
	}
	return value
}

func secretState(value string) string {
	if strings.TrimSpace(value) == "" {
		return "_not set_"
	}
	return "configured"
}
