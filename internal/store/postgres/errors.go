package postgres

import (
	"errors"

	"github.com/lib/pq"
)

// isInvalidUUID reports whether err is a Postgres invalid_text_representation
// error (SQLSTATE 22P02), i.e. a malformed value was passed to a uuid column.
// Callers map it to domain.ErrNotFound so invalid IDs surface as 404 instead
// of an unhandled 500 (consistent with valid-but-missing UUIDs).
func isInvalidUUID(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "22P02"
}
