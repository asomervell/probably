package sync

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// scanUUIDRows collects UUID values from a single-column rows result.
// The caller is responsible for closing rows.
func scanUUIDRows(rows pgx.Rows) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
