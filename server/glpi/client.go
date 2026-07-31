package glpi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 10 * time.Second

// Client is a reusable HTTP client for the GLPI REST API.
type Client struct {
	baseURL    string
	appToken   string
	userToken  string
	httpClient *http.Client

	sessionMu sync.Mutex
	session   string

	// rate limiting and retry policy
	rateLimitRPS int // requests per second (simple throttle)
	rateMu       sync.Mutex
	lastRequest  time.Time
	maxRetries   int
	backoffBase  time.Duration

	// debugLog, when set, receives diagnostics for every outbound GLPI request.
	debugLog func(msg string, keyvals ...interface{})
}

// GLPIClient describes the subset of GLPI functionality used by the plugin.
type GLPIClient interface {
	HealthCheck(ctx context.Context) (*HealthCheckResponse, error)
	KillSession(ctx context.Context) error

	CreateTicket(ctx context.Context, req CreateTicketRequest) (*CreateTicketResponse, error)
	GetTicket(ctx context.Context, id int) (*Ticket, error)
	UpdateTicket(ctx context.Context, id int, input map[string]interface{}) error
	DeleteTicket(ctx context.Context, id int) error
	AddFollowup(ctx context.Context, ticketID int, content string, isPrivate bool) error
	AddSolution(ctx context.Context, ticketID int, content string) error
	SearchTickets(ctx context.Context, filter TicketFilter) ([]TicketSummary, int, error)

	FindUserIDByEmail(ctx context.Context, email string) (int, error)
	SearchAssets(ctx context.Context, filter AssetFilter) ([]AssetSummary, int, error)
	SearchKnowledge(ctx context.Context, query string, categoryID, limit, page int) ([]KnowledgeSummary, int, error)
	SearchKnowledgeBaseCategories(ctx context.Context, limit int) ([]KnowbaseCategorySummary, int, error)
	GetTicketTimeline(ctx context.Context, ticketID int, request TimelinePageRequest) (*TimelinePage, error)
	UploadDocument(ctx context.Context, filename string, data []byte, ticketID int) (int, error)

	// Extended capabilities (v1.0): categories, KB article view, asset detail,
	// and document listing/download.
	SearchITILCategories(ctx context.Context, query string, limit int) ([]CategorySummary, int, error)
	GetKnowbaseItem(ctx context.Context, id int) (*KnowledgeArticle, error)
	GetAsset(ctx context.Context, itemType string, id int) (*AssetDetail, error)
	ListTicketDocuments(ctx context.Context, ticketID int) ([]DocumentInfo, error)
	GetDocumentContent(ctx context.Context, docID int) ([]byte, string, error)
}

// ResponseMetadata contains HTTP metadata for a successful GLPI response.
// Collection endpoints use Content-Range to expose server-side pagination.
type ResponseMetadata struct {
	StatusCode int
	Header     http.Header
}

// HealthCheckResponse represents the GLPI version payload returned by the health endpoint.
type HealthCheckResponse struct {
	Version string `json:"version"`
}

// ConfigError indicates that required configuration is missing.
type ConfigError struct {
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
	}
	return e.Message
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// AuthError indicates that the GLPI API rejected the supplied tokens.
type AuthError struct {
	Message string
	Err     error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
	}
	return e.Message
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// NetworkError indicates a transport-level failure while contacting GLPI.
type NetworkError struct {
	Message    string
	Err        error
	StatusCode int
}

func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
	}
	return e.Message
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// NotFoundError indicates that the requested GLPI resource does not exist.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

// NewClient creates a GLPI API client with configured auth headers and timeout.
func NewClient(baseURL, appToken, userToken string, httpClient *http.Client) (*Client, error) {
	trimmedBaseURL := strings.TrimSpace(baseURL)
	trimmedAppToken := strings.TrimSpace(appToken)
	trimmedUserToken := strings.TrimSpace(userToken)

	if trimmedBaseURL == "" {
		return nil, &ConfigError{Message: "GLPI URL is not configured"}
	}
	if trimmedAppToken == "" {
		return nil, &ConfigError{Message: "GLPI app token is not configured"}
	}
	if trimmedUserToken == "" {
		return nil, &ConfigError{Message: "GLPI user token is not configured"}
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	parsedURL, err := url.Parse(trimmedBaseURL)
	if err != nil {
		return nil, &ConfigError{Message: "GLPI URL is invalid", Err: err}
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, &ConfigError{Message: "GLPI URL must include http:// or https://"}
	}

	// sensible defaults for rate-limit and retry/backoff
	const defaultRPS = 5
	const defaultMaxRetries = 3
	const defaultBackoffBaseMS = 200

	return &Client{
		baseURL:      strings.TrimRight(trimmedBaseURL, "/"),
		appToken:     trimmedAppToken,
		userToken:    trimmedUserToken,
		httpClient:   httpClient,
		rateLimitRPS: defaultRPS,
		maxRetries:   defaultMaxRetries,
		backoffBase:  time.Millisecond * defaultBackoffBaseMS,
	}, nil
}

// doRequest performs an authenticated JSON request against the GLPI REST API.
// It ensures a session exists, retries exactly once on an expired session (401),
// and decodes the JSON response body into out when out is non-nil.
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
	return c.doRequestWithRetry(ctx, method, path, query, body, out, true, nil)
}

// doRequestWithResponse performs an authenticated request and exposes response
// metadata to callers that need pagination headers. The JSON and retry behavior
// is otherwise identical to doRequest.
func (c *Client) doRequestWithResponse(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}, metadata *ResponseMetadata) error {
	return c.doRequestWithRetry(ctx, method, path, query, body, out, true, metadata)
}

// doRequestRaw performs an authenticated request and returns the raw response
// body plus its content type without JSON decoding. Used for binary document
// downloads where the body must be proxied unchanged. Re-authenticates once if
// the cached session is rejected (401).
func (c *Client) doRequestRaw(ctx context.Context, method, path string, query url.Values) ([]byte, string, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, "", err
	}

	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
		if err != nil {
			return nil, &NetworkError{Message: "failed to build GLPI request", Err: err}
		}
		req.Header.Set("App-Token", c.appToken)
		req.Header.Set("Session-Token", c.currentSession())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", uuidNew())
		return req, nil
	}

	doOnce := func(req *http.Request) ([]byte, string, int, error) {
		c.acquireRate()
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, "", 0, &NetworkError{Message: "failed to reach GLPI", Err: err}
		}
		bodyBytes, readErr := io.ReadAll(resp.Body)
		contentType := resp.Header.Get("Content-Type")
		resp.Body.Close()

		if c.debugLog != nil {
			c.debugLog("GLPI request",
				"method", method,
				"url", fullURL,
				"status", resp.StatusCode,
				"body", truncateBody(string(bodyBytes)),
			)
		}
		if readErr != nil {
			return nil, "", resp.StatusCode, &NetworkError{Message: "failed to read GLPI response body", Err: readErr, StatusCode: resp.StatusCode}
		}
		return bodyBytes, contentType, resp.StatusCode, nil
	}

	req, err := buildReq()
	if err != nil {
		return nil, "", err
	}
	bodyBytes, contentType, status, err := doOnce(req)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusUnauthorized {
		c.clearSession()
		if err := c.ensureSession(ctx); err != nil {
			return nil, "", err
		}
		req2, buildErr := buildReq()
		if buildErr != nil {
			return nil, "", buildErr
		}
		bodyBytes, contentType, status, err = doOnce(req2)
		if err != nil {
			return nil, "", err
		}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, "", &NetworkError{Message: fmt.Sprintf("GLPI request failed (%d): %s", status, string(bodyBytes)), StatusCode: status}
	}
	return bodyBytes, contentType, nil
}

// acquireRate enforces a simple per-client rate limit by ensuring a minimum
// interval between requests. The mutex is held during the entire check-and-sleep
// window to prevent burst interleaving from concurrent goroutines.
func (c *Client) acquireRate() {
	if c.rateLimitRPS <= 0 {
		return
	}
	minInterval := time.Second / time.Duration(c.rateLimitRPS)
	c.rateMu.Lock()
	now := time.Now()
	delta := now.Sub(c.lastRequest)
	if delta < minInterval {
		sleep := minInterval - delta
		time.Sleep(sleep)
	}
	c.lastRequest = time.Now()
	c.rateMu.Unlock()
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}, allowRetry bool, metadata *ResponseMetadata) error {
	// Always ensure we have a valid session before performing requests.
	if err := c.ensureSession(ctx); err != nil {
		return err
	}

	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var payloadBytes []byte
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return &NetworkError{Message: "failed to encode GLPI request payload", Err: err}
		}
		payloadBytes = encoded
		reader = bytes.NewReader(payloadBytes)
	}

	// Build base request (we will clone it for retries to avoid reusing closed bodies).
	baseReq, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return &NetworkError{Message: "failed to build GLPI request", Err: err}
	}
	baseReq.Header.Set("App-Token", c.appToken)
	baseReq.Header.Set("Session-Token", c.currentSession())
	// default content type; callers that need multipart set their own when building request
	baseReq.Header.Set("Content-Type", "application/json")

	// Correlation: try to use any existing request ID from context; otherwise generate one
	reqID := requestIDFromContext(ctx)
	if reqID == "" {
		reqID = uuidNew()
	}
	baseReq.Header.Set("X-Request-ID", reqID)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// enforce rate limiting before each attempt
		c.acquireRate()

		// create a fresh request for this attempt
		var req *http.Request
		if payloadBytes != nil {
			req, err = http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(payloadBytes))
		} else {
			req, err = http.NewRequestWithContext(ctx, method, fullURL, nil)
		}
		if err != nil {
			return &NetworkError{Message: "failed to build GLPI request", Err: err}
		}
		// copy headers
		req.Header = baseReq.Header.Clone()

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// transport error — consider transient and retryable
			lastErr = &NetworkError{Message: "failed to reach GLPI", Err: err}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		// ensure body closed on each loop
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if c.debugLog != nil {
			c.debugLog("GLPI request",
				"method", method,
				"url", fullURL,
				"status", resp.StatusCode,
				"body", truncateBody(string(bodyBytes)),
			)
		}

		if metadata != nil {
			metadata.StatusCode = resp.StatusCode
			metadata.Header = resp.Header.Clone()
		}

		if readErr != nil {
			lastErr = &NetworkError{Message: "failed to read GLPI response body", Err: readErr, StatusCode: resp.StatusCode}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		// Handle 401 — session might be expired. Clear and retry once.
		if resp.StatusCode == http.StatusUnauthorized {
			c.clearSession()
			if allowRetry {
				// attempt a single retry after re-establishing session
				if err := c.ensureSession(ctx); err != nil {
					return err
				}
				// refresh header
				baseReq.Header.Set("Session-Token", c.currentSession())
				if attempt < c.maxRetries {
					time.Sleep(c.backoffBase * time.Duration(1<<attempt))
					continue
				}
			}
			return &AuthError{Message: fmt.Sprintf("GLPI authentication failed. Response: %s", string(bodyBytes))}
		}

		// Not found
		if resp.StatusCode == http.StatusNotFound {
			return &NotFoundError{Message: fmt.Sprintf("GLPI resource not found (%s)", path)}
		}

		// Retry on 5xx (server errors) as transient
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = &NetworkError{Message: fmt.Sprintf("GLPI server error (%d): %s", resp.StatusCode, string(bodyBytes)), StatusCode: resp.StatusCode}
			if attempt < c.maxRetries {
				time.Sleep(c.backoffBase * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		// Non-successful codes (4xx) are not retried except 401 handled above
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return &NetworkError{Message: fmt.Sprintf("GLPI request failed (%d): %s", resp.StatusCode, string(bodyBytes)), StatusCode: resp.StatusCode}
		}

		// Successful response
		if out != nil && len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, out); err != nil {
				return &NetworkError{Message: "failed to decode GLPI response", Err: err, StatusCode: resp.StatusCode}
			}
		}

		return nil
	}

	// exhausted retries
	if lastErr != nil {
		return lastErr
	}
	return &NetworkError{Message: "GLPI request failed after retries"}
}

// HealthCheck retrieves the GLPI version from the API using a session token flow.
func (c *Client) HealthCheck(ctx context.Context) (*HealthCheckResponse, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}

	// Only getGlpiConfig is a valid GLPI REST endpoint. GLPI has no
	// /apirest.php/healthcheck predefined endpoint: a request to it falls
	// through to the CommonDBTM itemtype handler and is rejected with HTTP 400
	// ERROR_RESOURCE_NOT_FOUND_NOR_COMMONDBTM. getGlpiConfig is a predefined
	// endpoint that returns the GLPI version in $CFG_GLPI['version'].
	endpoints := []string{"/apirest.php/getGlpiConfig"}
	var lastErr error
	for _, endpoint := range endpoints {
		var payload struct {
			Version     string `json:"version"`
			GLPIVersion string `json:"glpi_version"`

			Data struct {
				Version string `json:"version"`
			} `json:"data"`

			CfgGLPI struct {
				Version string `json:"version"`
			} `json:"cfg_glpi"`
		}

		err := c.doRequest(ctx, http.MethodGet, endpoint, nil, nil, &payload)
		if err != nil {
			var notFound *NotFoundError
			var netErr *NetworkError
			switch {
			case isAsNotFound(err, &notFound):
				continue
			case isAsNetwork(err, &netErr) && (netErr.StatusCode == http.StatusMethodNotAllowed || netErr.StatusCode == http.StatusBadRequest):
				// GLPI returns 400 ERROR_RESOURCE_NOT_FOUND_NOR_COMMONDBTM for
				// endpoints that are neither predefined nor CommonDBTM item
				// types; treat them as unavailable and continue to the next
				// endpoint.
				continue
			default:
				return nil, err
			}
		}

		version := strings.TrimSpace(payload.Version)
		if version == "" {
			version = strings.TrimSpace(payload.GLPIVersion)
		}
		if version == "" {
			version = strings.TrimSpace(payload.Data.Version)
		}
		if version == "" {
			version = strings.TrimSpace(payload.CfgGLPI.Version)
		}

		if version == "" {
			lastErr = &NetworkError{Message: "GLPI health check response did not include a version"}
			continue
		}
		return &HealthCheckResponse{Version: version}, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &NetworkError{Message: "GLPI health check endpoint was not available"}
}

func isAsNotFound(err error, target **NotFoundError) bool {
	nf, ok := err.(*NotFoundError)
	if ok {
		*target = nf
	}
	return ok
}

func isAsNetwork(err error, target **NetworkError) bool {
	ne, ok := err.(*NetworkError)
	if ok {
		*target = ne
	}
	return ok
}

// requestIDFromContext attempts to extract a request id stored in context.
// It looks for the common key "request_id" or "request-id"; callers may set
// a value using context.WithValue(ctx, "request_id", "...").
func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value("request_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if v := ctx.Value("request-id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// uuidNew generates a cryptographically strong request identifier. Correlation
// IDs are logged and forwarded to GLPI, so predictability must not make them a
// cross-request tracking or spoofing primitive.
func uuidNew() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	// Fallback: use timestamp + random to maintain uniqueness
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix()%1000)
}

// Exported setters allow the host/plugin to tune client behaviour at runtime.
func (c *Client) SetRateLimitRPS(rps int) {
	if rps < 0 {
		rps = 0
	}
	c.rateLimitRPS = rps
}

func (c *Client) SetMaxRetries(n int) {
	if n < 0 {
		n = 0
	}
	c.maxRetries = n
}

func (c *Client) SetBackoffBase(d time.Duration) {
	if d <= 0 {
		d = 100 * time.Millisecond
	}
	c.backoffBase = d
}

// SetDebugLogger installs a callback that receives per-request diagnostics
// (HTTP method, full URL, response status, response body) for every outbound
// GLPI API request. Intended for temporary troubleshooting.
func (c *Client) SetDebugLogger(fn func(msg string, keyvals ...interface{})) {
	c.debugLog = fn
}

// truncateBody caps a logged response body to a reasonable size so that
// collection responses do not flood the log.
func truncateBody(s string) string {
	const max = 2000
	if len(s) > max {
		return s[:max] + fmt.Sprintf("…(%d more bytes)", len(s)-max)
	}
	return s
}
