package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	queueIndexKeyPrefix = "glpi_retry_index"
	queueJobKeyPrefix   = "glpi_retry_job_"
)

// QueueJob represents a persisted retryable job.
type QueueJob struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	CreatedAt   int64           `json:"created_at"`
	UpdatedAt   int64           `json:"updated_at"`
	LastError   string          `json:"last_error,omitempty"`
}

// CreateTicketPayload stores the information necessary to replay a ticket creation.
type CreateTicketPayload struct {
	Request             glpi.CreateTicketRequest `json:"request"`
	RequesterMattermost string                   `json:"requester_mattermost_id,omitempty"`
	ChannelID           string                   `json:"channel_id,omitempty"`
	RequestID           string                   `json:"request_id,omitempty"`
}

// RetryQueue provides a simple KV-backed durable queue using Mattermost KV.
type RetryQueue struct {
	p            *Plugin
	indexKey     string
	jobKeyPrefix string
	workerCount  int
	maxAttempts  int
	backoffBase  time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	running      bool
}

// newRandomID generates a short random id for job keys.
func newRandomID() string {
	b := make([]byte, 8)
	_, _ = cryptorand.Read(b)
	return hex.EncodeToString(b)
}

// NewRetryQueue constructs a RetryQueue bound to the plugin instance.
func NewRetryQueue(p *Plugin, workerCount int, maxAttempts int, backoffBase time.Duration) *RetryQueue {
	if workerCount <= 0 {
		workerCount = 1
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if backoffBase <= 0 {
		backoffBase = 2 * time.Second
	}
	return &RetryQueue{
		p:            p,
		indexKey:     queueIndexKeyPrefix,
		jobKeyPrefix: queueJobKeyPrefix,
		workerCount:  workerCount,
		maxAttempts:  maxAttempts,
		backoffBase:  backoffBase,
		stopCh:       make(chan struct{}),
	}
}

// UpdateConfig updates retry queue configuration at runtime without
// stopping the queue or dropping pending jobs.
func (q *RetryQueue) UpdateConfig(cfg *Configuration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if cfg.RetryQueueWorkerCount > 0 {
		q.workerCount = cfg.RetryQueueWorkerCount
	}
	if cfg.RetryQueueMaxAttempts > 0 {
		q.maxAttempts = cfg.RetryQueueMaxAttempts
	}
	if cfg.RetryQueueBackoffBaseSeconds > 0 {
		q.backoffBase = time.Duration(cfg.RetryQueueBackoffBaseSeconds) * time.Second
	}
	q.p.API.LogInfo("GLPI retry queue configuration updated", "workers", q.workerCount, "max_attempts", q.maxAttempts)
}

// PendingCount returns the number of queued jobs currently waiting or retrying.
func (q *RetryQueue) PendingCount() int {
	if q == nil {
		return 0
	}
	raw, _ := q.p.API.KVGet(q.indexKey)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	return len(ids)
}

// Start launches background workers to process the queue.
func (q *RetryQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running {
		return
	}
	q.running = true
	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(i)
	}
	q.p.API.LogInfo("GLPI retry queue started", "workers", q.workerCount)
}

// Stop signals workers to exit and waits for them.
func (q *RetryQueue) Stop() {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}
	q.running = false
	q.mu.Unlock()
	close(q.stopCh)
	q.wg.Wait()
	q.p.API.LogInfo("GLPI retry queue stopped")
}

// EnqueueCreateTicket creates and persists a create_ticket job.
func (q *RetryQueue) EnqueueCreateTicket(ctx context.Context, payload CreateTicketPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	job := QueueJob{
		ID:          newRandomID(),
		Type:        "create_ticket",
		Payload:     raw,
		Attempts:    0,
		MaxAttempts: q.maxAttempts,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
	jobKey := q.jobKeyPrefix + job.ID
	jobBytes, _ := json.Marshal(job)
	if appErr := q.p.API.KVSet(jobKey, jobBytes); appErr != nil {
		return fmt.Errorf("kv set job failed: %s", appErr.Error())
	}
	// append to index
	if err := q.appendIndex(job.ID); err != nil {
		// best-effort: remove job if index update fails
		_ = q.p.API.KVDelete(jobKey)
		return err
	}
	q.p.API.LogInfo("Enqueued retry job", "id", job.ID, "type", job.Type)
	return nil
}

// appendIndex appends a job ID to the index array in KV.
func (q *RetryQueue) appendIndex(id string) error {
	q.p.API.LogDebug("Appending job to retry index", "id", id)
	raw, _ := q.p.API.KVGet(q.indexKey)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	ids = append(ids, id)
	newRaw, _ := json.Marshal(ids)
	if appErr := q.p.API.KVSet(q.indexKey, newRaw); appErr != nil {
		return fmt.Errorf("kv set index failed: %s", appErr.Error())
	}
	return nil
}

// popIndex removes and returns the first job id from the index.
func (q *RetryQueue) popIndex() (string, error) {
	raw, _ := q.p.API.KVGet(q.indexKey)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	if len(ids) == 0 {
		return "", nil
	}
	id := ids[0]
	ids = ids[1:]
	newRaw, _ := json.Marshal(ids)
	if appErr := q.p.API.KVSet(q.indexKey, newRaw); appErr != nil {
		return "", fmt.Errorf("kv set index failed: %s", appErr.Error())
	}
	return id, nil
}

// workerLoop runs one worker that continuously processes queued jobs.
func (q *RetryQueue) workerLoop(workerID int) {
	defer q.wg.Done()
	q.p.API.LogInfo("GLPI retry worker started", "worker", workerID)
	for {
		select {
		case <-q.stopCh:
			q.p.API.LogInfo("GLPI retry worker stopping", "worker", workerID)
			return
		default:
			// pop next job id
			id, err := q.popIndex()
			if err != nil {
				q.p.API.LogWarn("failed to pop index", "err", err.Error())
				time.Sleep(5 * time.Second)
				continue
			}
			if id == "" {
				// nothing to do
				time.Sleep(2 * time.Second)
				continue
			}
			jobKey := q.jobKeyPrefix + id
			raw, _ := q.p.API.KVGet(jobKey)
			if len(raw) == 0 {
				// missing job record; skip
				continue
			}
			var job QueueJob
			if err := json.Unmarshal(raw, &job); err != nil {
				q.p.API.LogWarn("malformed job payload, removing", "id", id, "err", err.Error())
				_ = q.p.API.KVDelete(jobKey)
				continue
			}
			// process job
			switch job.Type {
			case "create_ticket":
				q.processCreateTicket(&job)
			default:
				q.p.API.LogWarn("unknown job type, removing", "id", job.ID, "type", job.Type)
				_ = q.p.API.KVDelete(jobKey)
			}
		}
	}
}

// processCreateTicket replays a ticket creation request.
func (q *RetryQueue) processCreateTicket(job *QueueJob) {
	var payload CreateTicketPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		q.p.API.LogWarn("invalid create_ticket payload, removing", "id", job.ID, "err", err.Error())
		_ = q.p.API.KVDelete(q.jobKeyPrefix + job.ID)
		return
	}

	q.p.API.LogInfo("processing create_ticket job", "id", job.ID, "attempts", job.Attempts)
	client := q.p.GetGLPIClient()
	if client == nil {
		q.p.API.LogWarn("glpi client not initialized, requeueing job", "id", job.ID)
		q.requeueJobWithBackoff(job)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Replay through the centralized ticket service so retry follows the exact
	// same pipeline as the API and dialog paths (requester resolution, default
	// entity/category, metadata, ownership, notifications, websocket push).
	svc := NewTicketService(q.p)
	result, err := svc.CreateTicket(ctx, TicketInput{
		Subject:       payload.Request.Name,
		Content:       payload.Request.Content,
		Type:          payload.Request.Type,
		Priority:      payload.Request.Priority,
		Urgency:       payload.Request.Urgency,
		CategoryID:    payload.Request.ITILCategoryID,
		EntityID:      payload.Request.EntityID,
		RequesterID:   payload.Request.RequesterID,
		CreatorUserID: payload.RequesterMattermost,
		ChannelID:     payload.ChannelID,
		RequestID:     payload.RequestID,
	})
	if err == nil {
		res := result.Ticket
		q.p.API.LogInfo("queued ticket created", "job_id", job.ID, "ticket_id", res.ID)
		// notify channel or user if present
		if payload.ChannelID != "" && q.p.botUserID != "" {
			msg := fmt.Sprintf("✅ Queued ticket created: #%d", res.ID)
			post := &model.Post{
				UserId:    q.p.botUserID,
				ChannelId: payload.ChannelID,
				Message:   msg,
			}

			_, appErr := q.p.API.CreatePost(post)
			if appErr != nil {
				q.p.API.LogWarn(
					"failed to create Mattermost notification post",
					"job_id", job.ID,
					"err", appErr.Error(),
				)
			}
		}
		// remove job record
		_ = q.p.API.KVDelete(q.jobKeyPrefix + job.ID)
		return
	}

	q.p.API.LogWarn("create_ticket attempt failed, will retry if allowed", "job_id", job.ID, "err", err.Error(), "attempts", job.Attempts+1)
	job.Attempts++
	job.LastError = err.Error()
	job.UpdatedAt = time.Now().Unix()
	if job.Attempts >= job.MaxAttempts {
		q.p.API.LogWarn("create_ticket job reached max attempts, removing", "job_id", job.ID)
		_ = q.p.API.KVDelete(q.jobKeyPrefix + job.ID)
		// notify channel/user of permanent failure
		if payload.ChannelID != "" && q.p.botUserID != "" {
			msg := fmt.Sprintf("⚠️ Failed to create queued ticket after %d attempts: %s", job.Attempts, job.LastError)
			post := &model.Post{
				UserId:    q.p.botUserID,
				ChannelId: payload.ChannelID,
				Message:   msg,
			}

			_, appErr := q.p.API.CreatePost(post)
			if appErr != nil {
				q.p.API.LogWarn(
					"failed to create failure notification",
					"job_id", job.ID,
					"err", appErr.Error(),
				)
			}
		}
		return
	}

	// update job record
	jobKey := q.jobKeyPrefix + job.ID
	jb, _ := json.Marshal(job)
	if appErr := q.p.API.KVSet(jobKey, jb); appErr != nil {
		q.p.API.LogWarn("failed to update job record", "job_id", job.ID, "err", appErr.Error())
	}
	// exponential backoff sleep
	d := q.backoffBase * (1 << (job.Attempts - 1))
	if d > 1*time.Hour {
		d = 1 * time.Hour
	}
	time.Sleep(d)
	// re-insert job id at the head of the index so it will be retried again
	if err := q.prependIndex(job.ID); err != nil {
		q.p.API.LogWarn("failed to requeue job id into index", "job_id", job.ID, "err", err.Error())
	}
}

// requeueJobWithBackoff increments attempts and updates job record, then re-inserts into index.
func (q *RetryQueue) requeueJobWithBackoff(job *QueueJob) {
	job.Attempts++
	job.UpdatedAt = time.Now().Unix()
	job.LastError = "client-unavailable"
	jb, _ := json.Marshal(job)
	_ = q.p.API.KVSet(q.jobKeyPrefix+job.ID, jb)
	// sleep a little before requeuing
	time.Sleep(q.backoffBase)
	_ = q.prependIndex(job.ID)
}

// prependIndex inserts id at the front of the index.
func (q *RetryQueue) prependIndex(id string) error {
	raw, _ := q.p.API.KVGet(q.indexKey)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	ids = append([]string{id}, ids...)
	newRaw, _ := json.Marshal(ids)
	if appErr := q.p.API.KVSet(q.indexKey, newRaw); appErr != nil {
		return fmt.Errorf("kv set index failed: %s", appErr.Error())
	}
	return nil
}
