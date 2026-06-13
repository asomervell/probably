package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Thread represents a chat thread/conversation
type Thread struct {
	ID             uuid.UUID  `json:"id"`
	LedgerID       uuid.UUID  `json:"ledger_id"`
	UserID         uuid.UUID  `json:"user_id"`
	ParentThreadID *uuid.UUID `json:"parent_thread_id,omitempty"`
	Title          string     `json:"title,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Loaded separately
	Messages []Message `json:"messages,omitempty"`
}

// GetID implements ThreadStoreThread interface
func (t *Thread) GetID() uuid.UUID { return t.ID }

// GetTitle implements ThreadStoreThread interface
func (t *Thread) GetTitle() string { return t.Title }

// Message represents a single chat message
type Message struct {
	ID          uuid.UUID    `json:"id"`
	ThreadID    uuid.UUID    `json:"thread_id"`
	Role        string       `json:"role"` // "user" or "assistant"
	Content     string       `json:"content"`
	SQLQuery    string       `json:"-"` // Internal only, never exposed to client
	ResultsJSON *QueryResult `json:"-"` // Internal only
	CreatedAt   time.Time    `json:"created_at"`
}

// GetRole implements ThreadStoreMessage interface
func (m *Message) GetRole() string { return m.Role }

// GetContent implements ThreadStoreMessage interface
func (m *Message) GetContent() string { return m.Content }

// ThreadStore handles database operations for chat threads
type ThreadStore struct {
	pool *pgxpool.Pool
}

// NewThreadStore creates a new ThreadStore
func NewThreadStore(pool *pgxpool.Pool) *ThreadStore {
	return &ThreadStore{pool: pool}
}

// CreateThread creates a new chat thread
func (s *ThreadStore) CreateThread(ctx context.Context, ledgerID, userID uuid.UUID, parentThreadID *uuid.UUID) (*Thread, error) {
	thread := &Thread{
		ID:             uuid.New(),
		LedgerID:       ledgerID,
		UserID:         userID,
		ParentThreadID: parentThreadID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_threads (id, ledger_id, user_id, parent_thread_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, thread.ID, thread.LedgerID, thread.UserID, thread.ParentThreadID, thread.Title, thread.CreatedAt, thread.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return thread, nil
}

// GetThreadForUser retrieves a thread by ID, ensuring it belongs to the user and ledger
func (s *ThreadStore) GetThreadForUser(ctx context.Context, id, ledgerID, userID uuid.UUID) (*Thread, error) {
	thread := &Thread{}
	var parentThreadID *uuid.UUID

	err := s.pool.QueryRow(ctx, `
		SELECT id, ledger_id, user_id, parent_thread_id, title, created_at, updated_at
		FROM chat_threads
		WHERE id = $1 AND ledger_id = $2 AND user_id = $3
	`, id, ledgerID, userID).Scan(&thread.ID, &thread.LedgerID, &thread.UserID, &parentThreadID, &thread.Title, &thread.CreatedAt, &thread.UpdatedAt)
	if err != nil {
		return nil, err
	}

	thread.ParentThreadID = parentThreadID
	return thread, nil
}

// ListThreads lists threads for a user and ledger, ordered by most recently updated
func (s *ThreadStore) ListThreads(ctx context.Context, ledgerID, userID uuid.UUID, limit int) ([]Thread, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, user_id, parent_thread_id, title, created_at, updated_at
		FROM chat_threads
		WHERE ledger_id = $1 AND user_id = $2 AND parent_thread_id IS NULL
		ORDER BY updated_at DESC
		LIMIT $3
	`, ledgerID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var thread Thread
		var parentThreadID *uuid.UUID
		if err := rows.Scan(&thread.ID, &thread.LedgerID, &thread.UserID, &parentThreadID, &thread.Title, &thread.CreatedAt, &thread.UpdatedAt); err != nil {
			return nil, err
		}
		thread.ParentThreadID = parentThreadID
		threads = append(threads, thread)
	}

	return threads, rows.Err()
}

// DeleteThreadForUser deletes a thread, ensuring it belongs to the user and ledger
func (s *ThreadStore) DeleteThreadForUser(ctx context.Context, id, ledgerID, userID uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM chat_threads 
		WHERE id = $1 AND ledger_id = $2 AND user_id = $3
	`, id, ledgerID, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateThreadTitle updates the title of a thread
func (s *ThreadStore) UpdateThreadTitle(ctx context.Context, id uuid.UUID, title string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE chat_threads 
		SET title = $1, updated_at = NOW()
		WHERE id = $2
	`, title, id)
	return err
}

// UpdateThreadTimestamp updates the updated_at timestamp of a thread
func (s *ThreadStore) UpdateThreadTimestamp(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE chat_threads SET updated_at = NOW() WHERE id = $1
	`, id)
	return err
}

// AddMessage adds a message to a thread
func (s *ThreadStore) AddMessage(ctx context.Context, threadID uuid.UUID, role, content, sqlQuery string, results *QueryResult) (*Message, error) {
	msg := &Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Role:      role,
		Content:   content,
		SQLQuery:  sqlQuery,
		CreatedAt: time.Now(),
	}

	var resultsJSON []byte
	if results != nil {
		var err error
		resultsJSON, err = json.Marshal(results)
		if err != nil {
			return nil, err
		}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_messages (id, thread_id, role, content, sql_query, results_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, msg.ID, msg.ThreadID, msg.Role, msg.Content, sqlQuery, resultsJSON, msg.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Update thread timestamp
	if err := s.UpdateThreadTimestamp(ctx, threadID); err != nil {
		slog.WarnContext(ctx, "failed to update thread timestamp", "thread_id", threadID, "error", err)
	}

	return msg, nil
}

// GetMessages retrieves all messages for a thread, ordered by creation time
func (s *ThreadStore) GetMessages(ctx context.Context, threadID uuid.UUID) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, thread_id, role, content, sql_query, results_json, created_at
		FROM chat_messages
		WHERE thread_id = $1
		ORDER BY created_at ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var sqlQuery *string
		var resultsJSON []byte

		if err := rows.Scan(&msg.ID, &msg.ThreadID, &msg.Role, &msg.Content, &sqlQuery, &resultsJSON, &msg.CreatedAt); err != nil {
			return nil, err
		}

		if sqlQuery != nil {
			msg.SQLQuery = *sqlQuery
		}
		if len(resultsJSON) > 0 {
			var results QueryResult
			if err := json.Unmarshal(resultsJSON, &results); err == nil {
				msg.ResultsJSON = &results
			}
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetThreadWithMessagesForUser retrieves a thread with messages, ensuring ownership
func (s *ThreadStore) GetThreadWithMessagesForUser(ctx context.Context, id, ledgerID, userID uuid.UUID) (*Thread, error) {
	thread, err := s.GetThreadForUser(ctx, id, ledgerID, userID)
	if err != nil {
		return nil, err
	}

	messages, err := s.GetMessages(ctx, id)
	if err != nil {
		return nil, err
	}

	thread.Messages = messages
	return thread, nil
}

// CountMessages returns the number of messages in a thread
func (s *ThreadStore) CountMessages(ctx context.Context, threadID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM chat_messages WHERE thread_id = $1
	`, threadID).Scan(&count)
	return count, err
}
