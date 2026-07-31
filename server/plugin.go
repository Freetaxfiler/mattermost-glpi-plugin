package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
	"net/http"
	"strings"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/commands"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// PluginID must match the id declared in plugin.json.
const PluginID = "com.ntas.glpi"

const glpiUserCacheSeconds = 3600

// Plugin is the main server-side plugin entry point.
type Plugin struct {
	plugin.MattermostPlugin

	client            *pluginapi.Client
	configurationLock sync.RWMutex
	configuration     *Configuration
	glpiClient        glpi.GLPIClient
	botUserID         string
	// retry queue for durable retry of non-idempotent operations
	retryQueue        *RetryQueue
}

// OnActivate is called when the plugin is activated.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	p.configuration = &Configuration{}

	botUserID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    "glpi",
		DisplayName: "GLPI",
		Description: "GLPI IT support bot",
	})
	if err != nil {
		p.API.LogWarn("failed to ensure GLPI bot; notifications will be disabled", "err", err.Error())
	} else {
		p.botUserID = botUserID
	}

	if err := p.API.RegisterCommand(commands.GetCommand()); err != nil {
		p.API.LogError("failed to register /glpi command", "err", err.Error())
		return err
	}

	if err := p.LoadConfiguration(); err != nil {
		p.API.LogError("failed to load plugin configuration", "err", err.Error())
		return err
	}

	if err := p.initializeGLPIClient(); err != nil {
		p.API.LogWarn("failed to initialize GLPI client", "err", err.Error())
	}

	// initialize and start retry queue
	p.retryQueue = newRetryQueueFromConfig(p)
	p.retryQueue.Start()

	return nil
}

// OnConfigurationChange is invoked by Mattermost when plugin settings are updated.
func (p *Plugin) OnConfigurationChange() error {
	if err := p.LoadConfiguration(); err != nil {
		p.API.LogError("failed to reload plugin configuration", "err", err.Error())
		return err
	}

	if err := p.initializeGLPIClient(); err != nil {
		p.API.LogWarn("failed to initialize GLPI client after configuration change", "err", err.Error())
	}

	// update retry queue configuration without dropping pending jobs
	if p.retryQueue != nil {
		config := p.currentConfiguration()
		if config != nil {
			p.retryQueue.UpdateConfig(config)
		}
	} else {
		p.retryQueue = newRetryQueueFromConfig(p)
		p.retryQueue.Start()
	}

	return nil
}

// ExecuteCommand is called when a registered slash command is invoked.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (resp *model.CommandResponse, appErr *model.AppError) {
	defer func() {
		if r := recover(); r != nil {
			p.API.LogError(
				"PANIC in ExecuteCommand",
				"panic", fmt.Sprintf("%v", r),
			)
			resp = &model.CommandResponse{
				ResponseType: model.CommandResponseTypeEphemeral,
				Text:         "An unexpected error occurred while executing the command.",
			}
		}
	}()

	if p.API == nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Plugin API is not initialized.",
		}, nil
	}

	if args == nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "No command arguments were provided.",
		}, nil
	}

	return commands.ExecuteCommand(p, args)
}

// GetConfiguration returns the current plugin configuration in a thread-safe
// manner for command use.
func (p *Plugin) GetConfiguration() *commands.ConfigView {
	config := p.currentConfiguration()
	if config == nil {
		return &commands.ConfigView{}
	}
	return &commands.ConfigView{
		GLPIURL:               config.GLPIURL,
		AppToken:              config.AppToken,
		UserToken:             config.UserToken,
		DefaultEntity:         config.DefaultEntity,
		DefaultCategory:       config.DefaultCategory,
		NotificationChannelID: config.NotificationChannelID,
		EnableDebug:           config.EnableDebug,
	}
}

// GetGLPIClient returns the initialized GLPI client for command usage.
func (p *Plugin) GetGLPIClient() glpi.GLPIClient {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()
	return p.glpiClient
}

// GetGLPIUserID resolves (and caches) the GLPI user ID matching the Mattermost
// user's email address.
func (p *Plugin) GetGLPIUserID(mattermostUserID string) (int, error) {
	kvKey := "glpi_uid_" + mattermostUserID
	if data, appErr := p.API.KVGet(kvKey); appErr == nil && len(data) > 0 {
		if id, err := strconv.Atoi(string(data)); err == nil && id > 0 {
			return id, nil
		}
	}

	user, appErr := p.API.GetUser(mattermostUserID)
	if appErr != nil {
		return 0, fmt.Errorf("failed to load Mattermost user: %s", appErr.Error())
	}

	client := p.GetGLPIClient()
	if client == nil {
		return 0, fmt.Errorf("GLPI client is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	glpiUserID, err := client.FindUserIDByEmail(ctx, user.Email)
	if err != nil {
		return 0, err
	}

	if appErr := p.API.KVSetWithExpiry(kvKey, []byte(strconv.Itoa(glpiUserID)), glpiUserCacheSeconds); appErr != nil {
		p.API.LogWarn("failed to cache GLPI user id", "err", appErr.Error())
	}

	return glpiUserID, nil
}

// IsSystemAdmin reports whether the given Mattermost user is a system admin.
func (p *Plugin) IsSystemAdmin(mattermostUserID string) bool {
	return p.API.HasPermissionTo(mattermostUserID, model.PermissionManageSystem)
}

// LatestFileAttachment returns the newest file the user posted in the channel.
func (p *Plugin) LatestFileAttachment(mattermostUserID, channelID string) (string, []byte, error) {
	postList, appErr := p.API.GetPostsForChannel(channelID, 0, 30)
	if appErr != nil {
		return "", nil, fmt.Errorf("failed to load channel posts: %s", appErr.Error())
	}
	if postList == nil {
		return "", nil, fmt.Errorf("no recent posts found in this channel")
	}

	for _, postID := range postList.Order {
		post, ok := postList.Posts[postID]
		if !ok || post == nil {
			continue
		}
		if post.UserId != mattermostUserID || len(post.FileIds) == 0 {
			continue
		}

		fileID := post.FileIds[0]
		info, appErr := p.API.GetFileInfo(fileID)
		if appErr != nil {
			return "", nil, fmt.Errorf("failed to load file info: %s", appErr.Error())
		}
		data, appErr := p.API.GetFile(fileID)
		if appErr != nil {
			return "", nil, fmt.Errorf("failed to download file: %s", appErr.Error())
		}

		// Validate size and mime
		config := p.currentConfiguration()
		maxUpload := 10 * 1024 * 1024 // 10MB default
		if config != nil && config.MaxUploadSizeBytes > 0 {
			maxUpload = config.MaxUploadSizeBytes
		}
		if len(data) > maxUpload {
			return "", nil, fmt.Errorf("file exceeds maximum allowed size (%d bytes)", maxUpload)
		}

		allowedMIMEs := glpi.DefaultAllowedMIMEs()
		if config != nil && len(config.AllowedMIMEs) > 0 {
			allowedMIMEs = map[string]bool{}
			for _, m := range config.AllowedMIMEs {
				allowedMIMEs[strings.TrimSpace(m)] = true
			}
		}
		if _, err := glpi.ValidateFileMIME(data, allowedMIMEs); err != nil {
			return "", nil, fmt.Errorf("file validation failed: %w", err)
		}

		// sanitize filename using shared helper
		safe := glpi.SanitizeFilename(info.Name)
		if safe == "" {
			safe = "attachment"
		}

		return safe, data, nil
	}

	return "", nil, fmt.Errorf("no file uploads from you found in the last 30 posts of this channel")
}

// OnDeactivate is called when the plugin is deactivated.
func (p *Plugin) OnDeactivate() error {
	if err := p.API.UnregisterCommand("", commands.Trigger); err != nil {
		p.API.LogError("failed to unregister /glpi command", "err", err.Error())
		return err
	}

	if p.glpiClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.glpiClient.KillSession(ctx); err != nil {
			p.API.LogWarn("failed to kill GLPI session", "err", err.Error())
		}
	}

	// stop retry queue if running
	if p.retryQueue != nil {
		p.retryQueue.Stop()
		p.retryQueue = nil
	}

	p.configurationLock.Lock()
	p.glpiClient = nil
	p.configurationLock.Unlock()
	return nil
}

func (p *Plugin) initializeGLPIClient() error {
	config := p.currentConfiguration()
	if config == nil {
		return &glpi.ConfigError{Message: "GLPI configuration is unavailable"}
	}

	// Build a custom HTTP client if a request timeout is configured.
	var httpClient *http.Client
	if config.RequestTimeoutSeconds > 0 {
		httpClient = &http.Client{Timeout: time.Duration(config.RequestTimeoutSeconds) * time.Second}
	}

	client, err := glpi.NewClient(config.GLPIURL, config.AppToken, config.UserToken, httpClient)
	if err != nil {
		return err
	}

	// Tune client behaviour with configuration-driven knobs when present.
	if config.RateLimitRPS > 0 {
		client.SetRateLimitRPS(config.RateLimitRPS)
	}
	if config.RequestTimeoutSeconds > 0 {
		client.SetBackoffBase(time.Duration(200) * time.Millisecond)
	}

	// Per-request GLPI diagnostics (HTTP method, full URL, response status,
	// response body) are emitted only when debug logging is enabled. This has
	// no effect on production log output when EnableDebug is off.
	if config.EnableDebug {
		client.SetDebugLogger(func(msg string, keyvals ...interface{}) {
			p.API.LogInfo(msg, keyvals...)
		})
	}

	p.configurationLock.Lock()
	p.glpiClient = client
	p.configurationLock.Unlock()

	p.API.LogInfo("GLPI client configured", "url", config.GLPIURL)
	return nil
}

func (p *Plugin) currentConfiguration() *Configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return nil
	}
	return p.configuration.Clone()
}

// newRetryQueueFromConfig builds a RetryQueue using the current plugin configuration.
func newRetryQueueFromConfig(p *Plugin) *RetryQueue {
	config := p.currentConfiguration()
	workerCount := 1
	maxAttempts := 5
	backoffBase := 2 * time.Second
	if config != nil {
		if config.RetryQueueWorkerCount > 0 {
			workerCount = config.RetryQueueWorkerCount
		}
		if config.RetryQueueMaxAttempts > 0 {
			maxAttempts = config.RetryQueueMaxAttempts
		}
		if config.RetryQueueBackoffBaseSeconds > 0 {
			backoffBase = time.Duration(config.RetryQueueBackoffBaseSeconds) * time.Second
		}
	}
	return NewRetryQueue(p, workerCount, maxAttempts, backoffBase)
}

// EnqueueCreateTicket enqueues a create-ticket job for durable retry.
func (p *Plugin) EnqueueCreateTicket(ctx context.Context, req CreateTicketPayload) error {
	if p.retryQueue == nil {
		return fmt.Errorf("retry queue not initialized")
	}
	return p.retryQueue.EnqueueCreateTicket(ctx, req)
}
