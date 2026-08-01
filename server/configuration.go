package main

import (
	"fmt"
	"strings"
)

// Configuration holds the plugin settings used for GLPI integration.
type Configuration struct {
	GLPIURL               string   `json:"glpi_url"`
	AppToken              string   `json:"app_token"`
	UserToken             string   `json:"user_token"`
	DefaultEntity         string   `json:"default_entity"`
	DefaultCategory       string   `json:"default_category"`
	WebhookSecret         string   `json:"webhook_secret"`
	NotificationChannelID string   `json:"notification_channel_id"`
	EnableDebug           bool     `json:"enable_debug"`
	EnableUserMapping     bool     `json:"enable_user_mapping"`

	// Production hardening options (optional)
	MaxUploadSizeBytes           int      `json:"max_upload_size_bytes"`
	AllowedMIMEs                []string `json:"allowed_mimes"`
	RateLimitRPS                int      `json:"rate_limit_rps"`
	RequestTimeoutSeconds       int      `json:"request_timeout_seconds"`
	WebhookReplayWindowSeconds  int      `json:"webhook_replay_window_seconds"`
	// Retry queue configuration
	RetryQueueWorkerCount        int      `json:"retry_queue_worker_count"`
	RetryQueueMaxAttempts        int      `json:"retry_queue_max_attempts"`
	RetryQueueBackoffBaseSeconds int      `json:"retry_queue_backoff_base_seconds"`
}

// Clone returns a deep copy of the configuration.
func (c *Configuration) Clone() *Configuration {
	if c == nil {
		return &Configuration{}
	}

	clone := *c
	// deep copy slices
	if c.AllowedMIMEs != nil {
		clone.AllowedMIMEs = make([]string, len(c.AllowedMIMEs))
		copy(clone.AllowedMIMEs, c.AllowedMIMEs)
	}
	return &clone
}

// SetConfiguration updates the plugin configuration in a thread-safe manner.
func (p *Plugin) SetConfiguration(configuration *Configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration == nil {
		p.configuration = &Configuration{}
		return
	}

	p.configuration = configuration.Clone()
}

// Validate checks the configuration values and returns a warning-style error
// when required values are missing.
func (c *Configuration) Validate() error {
	if c == nil {
		return fmt.Errorf("configuration is nil")
	}

	if strings.TrimSpace(c.GLPIURL) == "" {
		return fmt.Errorf("glpi URL is empty")
	}

	if !strings.HasPrefix(c.GLPIURL, "http://") && !strings.HasPrefix(c.GLPIURL, "https://") {
		return fmt.Errorf("glpi URL must start with http:// or https://")
	}

	if strings.TrimSpace(c.AppToken) == "" {
		return fmt.Errorf("app token is empty")
	}
	if strings.TrimSpace(c.UserToken) == "" {
		return fmt.Errorf("user token is empty")
	}

	// Optional checks
	if strings.TrimSpace(c.WebhookSecret) != "" && len(c.WebhookSecret) < 8 {
		return fmt.Errorf("webhook secret must be at least 8 characters")
	}

	if c.RateLimitRPS < 0 {
		return fmt.Errorf("rate_limit_rps cannot be negative")
	}
	if c.MaxUploadSizeBytes < 0 {
		return fmt.Errorf("max_upload_size_bytes cannot be negative")
	}
	if c.RequestTimeoutSeconds < 0 {
		return fmt.Errorf("request_timeout_seconds cannot be negative")
	}
	if c.WebhookReplayWindowSeconds < 0 {
		return fmt.Errorf("webhook_replay_window_seconds cannot be negative")
	}
	if c.RetryQueueWorkerCount < 0 {
		return fmt.Errorf("retry_queue_worker_count cannot be negative")
	}
	if c.RetryQueueMaxAttempts < 0 {
		return fmt.Errorf("retry_queue_max_attempts cannot be negative")
	}
	if c.RetryQueueBackoffBaseSeconds < 0 {
		return fmt.Errorf("retry_queue_backoff_base_seconds cannot be negative")
	}

	return nil
}

// Redacted returns a copy of the configuration with sensitive values masked
// for safe logging.
func (c *Configuration) Redacted() *Configuration {
	if c == nil {
		return &Configuration{}
	}

	clone := c.Clone()
	clone.AppToken = maskSecret(clone.AppToken)
	clone.UserToken = maskSecret(clone.UserToken)
	clone.WebhookSecret = maskSecret(clone.WebhookSecret)
	// do not redact AllowedMIMEs or numeric config
	return clone
}

func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}

	return value[:3] + strings.Repeat("*", len(value)-6) + value[len(value)-3:]
}

// LoadConfiguration loads configuration from Mattermost into the plugin state.
func (p *Plugin) LoadConfiguration() error {
	configuration := &Configuration{}
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return err
	}

	if err := configuration.Validate(); err != nil {
		p.API.LogWarn("plugin configuration validation failed", "err", err.Error(), "config", configuration.Redacted())
		// Return the validation error so it is surfaced in the System Console.
		// The partial configuration is still stored so admins can build upon it.
		p.SetConfiguration(configuration)
		return err
	}

	p.SetConfiguration(configuration)
	return nil
}
