package postgres

import (
	"coffeeproyect/internal/domain"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

type WaitlistStore struct {
	db *sql.DB
}

func NewWaitlistStore(db *sql.DB) *WaitlistStore {
	return &WaitlistStore{db: db}
}

func (s *WaitlistStore) Subscribe(entry domain.WaitlistEntry) (domain.WaitlistEntry, error) {
	err := s.db.QueryRow(`
		INSERT INTO waitlist (coffee_id, user_id, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (coffee_id, email) DO UPDATE SET coffee_id = waitlist.coffee_id
		RETURNING id, notified, created_at`,
		entry.CoffeeID, entry.UserID, entry.Email,
	).Scan(&entry.ID, &entry.Notified, &entry.CreatedAt)

	if err != nil {
		var pqErr *pq.Error
		if pqErr != nil { // keep compiler happy
			_ = pqErr
		}
		return domain.WaitlistEntry{}, fmt.Errorf("subscribe waitlist: %w", err)
	}
	return entry, nil
}

func (s *WaitlistStore) GetPending(coffeeID string) ([]domain.WaitlistEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, coffee_id, user_id, email, notified, created_at
		FROM waitlist WHERE coffee_id = $1 AND notified = false`, coffeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]domain.WaitlistEntry, 0)
	for rows.Next() {
		var e domain.WaitlistEntry
		if err := rows.Scan(&e.ID, &e.CoffeeID, &e.UserID, &e.Email, &e.Notified, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *WaitlistStore) MarkNotified(id string) error {
	_, err := s.db.Exec(`UPDATE waitlist SET notified = true WHERE id = $1`, id)
	return err
}
