package glpi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// currentSession returns the cached session token in a thread-safe manner.
func (c *Client) currentSession() string {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	return c.session
}

// clearSession discards the cached session token so the next request re-authenticates.
func (c *Client) clearSession() {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	c.session = ""
}

// ensureSession initializes a GLPI API session if one is not already cached.
func (c *Client) ensureSession(ctx context.Context) error {
	c.sessionMu.Lock()
	if strings.TrimSpace(c.session) != "" {
		// already set; nothing to do
		c.sessionMu.Unlock()
		return nil
	}
	// release lock while performing network calls; we will re-acquire when storing session
	c.sessionMu.Unlock()

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// rate control
		c.acquireRate()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/apirest.php/initSession", nil)
		if err != nil {
			return &NetworkError{Message: "failed to build GLPI session request", Err: err}
		}
		req.Header.Set("App-Token", c.appToken)
		req.Header.Set("Authorization", "user_token "+c.userToken)
		req.Header.Set("Content-Type", "application/json")
		// request id header
		req.Header.Set("X-Request-ID", uuidNew())

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &NetworkError{Message: "failed to reach GLPI during session initialization", Err: err}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if c.debugLog != nil {
			c.debugLog("GLPI request",
				"method", http.MethodGet,
				"url", c.baseURL+"/apirest.php/initSession",
				"status", resp.StatusCode,
				"body", truncateBody(string(bodyBytes)),
			)
		}

		if readErr != nil {
			lastErr = &NetworkError{Message: "failed to read GLPI session response body", Err: readErr}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return &AuthError{
				Message: fmt.Sprintf(
					"GLPI authentication failed (401). Response: %s",
					string(bodyBytes),
				),
			}
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			lastErr = &NetworkError{
				Message:    fmt.Sprintf("GLPI session initialization failed with status %d", resp.StatusCode),
				StatusCode: resp.StatusCode,
			}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		var payload struct {
			SessionToken string `json:"session_token"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return &NetworkError{Message: "failed to decode GLPI session response", Err: err}
		}
		st := strings.TrimSpace(payload.SessionToken)
		if st == "" {
			lastErr = &NetworkError{Message: "GLPI session initialization did not return a session token"}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		// store the session token
		c.sessionMu.Lock()
		c.session = st
		c.sessionMu.Unlock()
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return &NetworkError{Message: "GLPI session initialization failed after retries"}
}

// KillSession closes the cached GLPI session.
func (c *Client) KillSession(ctx context.Context) error {
	session := c.currentSession()
	if strings.TrimSpace(session) == "" {
		return nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/apirest.php/killSession", nil)
	if err != nil {
		return &NetworkError{Message: "failed to build GLPI session teardown request", Err: err}
	}
	request.Header.Set("App-Token", c.appToken)
	request.Header.Set("Session-Token", session)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return &NetworkError{Message: "failed to reach GLPI during session teardown", Err: err}
	}
	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	if c.debugLog != nil {
		c.debugLog("GLPI request",
			"method", http.MethodGet,
			"url", c.baseURL+"/apirest.php/killSession",
			"status", response.StatusCode,
			"body", truncateBody(string(bodyBytes)),
		)
	}

	c.clearSession()
	return nil
}
