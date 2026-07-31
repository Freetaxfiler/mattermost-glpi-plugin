package glpi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// DefaultAllowedMIMEs returns the default set of allowed MIME types for file uploads.
func DefaultAllowedMIMEs() map[string]bool {
	return map[string]bool{
		"image/jpeg":               true,
		"image/png":                true,
		"application/pdf":          true,
		"text/plain; charset=utf-8": true,
		"text/plain":               true,
	}
}

// ValidateFileMIME checks whether the given data's detected MIME type is allowed.
// The allowed map may be nil to use the default set. An empty allowed map rejects all types.
func ValidateFileMIME(data []byte, allowed map[string]bool) (string, error) {
	probeLen := 512
	if len(data) < probeLen {
		probeLen = len(data)
	}
	mimeType := http.DetectContentType(data[:probeLen])
	if allowed == nil {
		allowed = DefaultAllowedMIMEs()
	}
	if !allowed[mimeType] {
		return "", fmt.Errorf("file MIME type not allowed: %s", mimeType)
	}
	return mimeType, nil
}

// UploadDocument uploads a file to GLPI as a Document and, when ticketID > 0,
// links the created document to that ticket. It returns the new document ID.
func (c *Client) UploadDocument(ctx context.Context, filename string, data []byte, ticketID int) (int, error) {
	if filename == "" {
		return 0, &ConfigError{Message: "document filename is empty"}
	}
	if len(data) == 0 {
		return 0, &ConfigError{Message: "document content is empty"}
	}

	// Basic upload validation/sanitisation before calling GLPI.
	safeName := SanitizeFilename(filename)
	if safeName == "" {
		return 0, &ConfigError{Message: "document filename is invalid after sanitization"}
	}

	// Max upload size (default 10MB) — conservative to protect GLPI and plugin runtime.
	const defaultMaxUpload = 10 * 1024 * 1024
	maxUpload := defaultMaxUpload
	if len(data) > maxUpload {
		return 0, &NetworkError{Message: fmt.Sprintf("document exceeds maximum allowed size (%d bytes)", maxUpload)}
	}

	// MIME type validation using shared helper
	if _, err := ValidateFileMIME(data, nil); err != nil {
		return 0, &NetworkError{Message: err.Error()}
	}

	documentID, err := c.uploadDocumentOnce(ctx, safeName, data, true)
	if err != nil {
		return 0, err
	}

	if ticketID > 0 {
		linkPayload := map[string]interface{}{
			"input": map[string]interface{}{
				"documents_id": documentID,
				"itemtype":     "Ticket",
				"items_id":     ticketID,
			},
		}
		if err := c.doRequest(ctx, http.MethodPost, "/apirest.php/Document_Item", nil, linkPayload, nil); err != nil {
			return documentID, fmt.Errorf("document %d uploaded but linking to ticket %d failed: %w", documentID, ticketID, err)
		}
	}

	return documentID, nil
}

func (c *Client) uploadDocumentOnce(ctx context.Context, filename string, data []byte, allowRetry bool) (int, error) {
	if err := c.ensureSession(ctx); err != nil {
		return 0, err
	}

	manifest, err := json.Marshal(map[string]interface{}{
		"input": map[string]interface{}{
			"name":      filename,
			"_filename": []string{filename},
		},
	})
	if err != nil {
		return 0, &NetworkError{Message: "failed to encode document upload manifest", Err: err}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("uploadManifest", string(manifest)); err != nil {
		return 0, &NetworkError{Message: "failed to build document upload form", Err: err}
	}

	filePart, err := writer.CreateFormFile("filename[0]", filename)
	if err != nil {
		return 0, &NetworkError{Message: "failed to build document upload form file", Err: err}
	}
	if _, err := filePart.Write(data); err != nil {
		return 0, &NetworkError{Message: "failed to write document upload data", Err: err}
	}
	if err := writer.Close(); err != nil {
		return 0, &NetworkError{Message: "failed to finalize document upload form", Err: err}
	}

	// Prepare a reusable byte slice for the request body so retries can recreate readers.
	bodyBytes := body.Bytes()

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// rate control between attempts
		c.acquireRate()

		request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/apirest.php/Document", bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, &NetworkError{Message: "failed to build GLPI document request", Err: err}
		}
		request.Header.Set("App-Token", c.appToken)
		request.Header.Set("Session-Token", c.currentSession())
		request.Header.Set("Content-Type", writer.FormDataContentType())
		// attach request id
		request.Header.Set("X-Request-ID", uuidNew())

		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr = &NetworkError{Message: "failed to reach GLPI during document upload", Err: err}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return 0, lastErr
		}
		responseBody, err := io.ReadAll(response.Body)
		response.Body.Close()

		if c.debugLog != nil {
			c.debugLog("GLPI request",
				"method", http.MethodPost,
				"url", c.baseURL+"/apirest.php/Document",
				"status", response.StatusCode,
				"body", truncateBody(string(responseBody)),
			)
		}

		if err != nil {
			lastErr = &NetworkError{Message: "failed to read GLPI document response", Err: err}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return 0, lastErr
		}

		if response.StatusCode == http.StatusUnauthorized {
			c.clearSession()
			if allowRetry {
				// retry once after re-auth
				if err := c.ensureSession(ctx); err != nil {
					return 0, err
				}
				if attempt < c.maxRetries {
					time.Sleep(c.backoffBase * time.Duration(1<<attempt))
					continue
				}
			}
			return 0, &AuthError{Message: fmt.Sprintf("GLPI authentication failed during document upload. Response: %s", string(responseBody))}
		}
		if response.StatusCode >= 500 && response.StatusCode < 600 {
			lastErr = &NetworkError{
				Message:    fmt.Sprintf("GLPI document upload failed (%d): %s", response.StatusCode, string(responseBody)),
				StatusCode: response.StatusCode,
			}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return 0, lastErr
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return 0, &NetworkError{
				Message:    fmt.Sprintf("GLPI document upload failed (%d): %s", response.StatusCode, string(responseBody)),
				StatusCode: response.StatusCode,
			}
		}

		var result struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return 0, &NetworkError{Message: "failed to decode GLPI document response", Err: err}
		}
		if result.ID == 0 {
			return 0, &NetworkError{Message: fmt.Sprintf("GLPI document upload returned no ID. Response: %s", string(responseBody))}
		}

		return result.ID, nil
	}

	if lastErr != nil {
		return 0, lastErr
	}
	return 0, &NetworkError{Message: "GLPI document upload failed after retries"}
}

// SanitizeFilename removes path separators and dangerous characters from a filename.
func SanitizeFilename(name string) string {
	// trim spaces
	n := strings.TrimSpace(name)
	if n == "" {
		return ""
	}
	// Remove any directory separators and control characters
	cleaned := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return -1
		}
		// allow alphanum and a few safe punctuation
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		switch r {
		case '.', '-', '_', ' ':
			return r
		default:
			return -1
		}
	}, n)

	// Trim leading dots to avoid hidden files
	cleaned = strings.TrimLeft(cleaned, ".")
	if len(cleaned) > 200 {
		cleaned = cleaned[:200]
	}
	return cleaned
}
