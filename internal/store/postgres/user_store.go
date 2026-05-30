package postgres

import (
	"coffeeproyect/internal/domain"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(user domain.User) (domain.User, error) {
	err := s.db.QueryRow(
		`INSERT INTO users (name, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		user.Name, user.Email, user.PasswordHash,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.User{}, domain.ErrEmailAlreadyExists
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *UserStore) GetByEmail(email string) (domain.User, error) {
	var u domain.User
	err := s.db.QueryRow(
		`SELECT id, name, email, password_hash, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (s *UserStore) GetByID(id string) (domain.User, error) {
	var u domain.User
	err := s.db.QueryRow(
		`SELECT id, name, email, password_hash, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (s *UserStore) Update(user domain.User) (domain.User, error) {
	res, err := s.db.Exec(
		`UPDATE users SET name = $1 WHERE id = $2`,
		user.Name, user.ID,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.User{}, domain.ErrNotFound
	}
	return user, nil
}

func (s *UserStore) AddAddress(address domain.Address) (domain.Address, error) {
	if address.IsDefault {
		_, err := s.db.Exec(
			`UPDATE addresses SET is_default = false WHERE user_id = $1`,
			address.UserID,
		)
		if err != nil {
			return domain.Address{}, fmt.Errorf("unset default addresses: %w", err)
		}
	}

	err := s.db.QueryRow(
		`INSERT INTO addresses (user_id, label, street, city, department, country, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		address.UserID, address.Label, address.Street,
		address.City, address.Department, address.Country, address.IsDefault,
	).Scan(&address.ID)

	if err != nil {
		return domain.Address{}, fmt.Errorf("add address: %w", err)
	}
	return address, nil
}

func (s *UserStore) GetAddresses(userID string) ([]domain.Address, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, label, street, city, department, country, is_default
		 FROM addresses WHERE user_id = $1 ORDER BY is_default DESC, id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get addresses: %w", err)
	}
	defer rows.Close()

	addresses := make([]domain.Address, 0)
	for rows.Next() {
		var a domain.Address
		if err := rows.Scan(&a.ID, &a.UserID, &a.Label, &a.Street,
			&a.City, &a.Department, &a.Country, &a.IsDefault); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

func (s *UserStore) DeleteAddress(id, userID string) error {
	res, err := s.db.Exec(
		`DELETE FROM addresses WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete address: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
