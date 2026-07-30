package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

func executeStatus(ctx context.Context, p PluginExecutor, config *ConfigView) (*model.CommandResponse, *model.AppError) {
	if strings.TrimSpace(config.GLPIURL) == "" {
		return responseText("GLPI is not configured. Set the GLPI URL, App Token, and User Token in the plugin settings."), nil
	}
	if strings.TrimSpace(config.AppToken) == "" || strings.TrimSpace(config.UserToken) == "" {
		return responseText("GLPI authentication is not configured. Add both the App Token and User Token in the plugin settings."), nil
	}

	client := p.GetGLPIClient()
	if client == nil {
		return responseText("GLPI client is not initialized yet."), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := client.HealthCheck(ctx)
	if err != nil {
		var cfgErr *glpi.ConfigError
		var authErr *glpi.AuthError
		var netErr *glpi.NetworkError
		switch {
		case errors.As(err, &cfgErr):
			return responseText(fmt.Sprintf("GLPI configuration error: %v", err)), nil
		case errors.As(err, &authErr):
			return responseText(fmt.Sprintf("GLPI authentication error: %v", err)), nil
		case errors.As(err, &netErr):
			return responseText(fmt.Sprintf("GLPI connection error: %v", err)), nil
		default:
			return responseText(fmt.Sprintf("GLPI status check failed: %v", err)), nil
		}
	}

	return responseText(fmt.Sprintf("GLPI is reachable. Version: %s", result.Version)), nil
}
