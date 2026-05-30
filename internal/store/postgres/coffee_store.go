package postgres

import (
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/store"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type CoffeeStore struct {
	db *sql.DB
}

func NewCoffeeStore(db *sql.DB) *CoffeeStore {
	return &CoffeeStore{db: db}
}

const selectCoffeeFields = `
	SELECT
		c.id, c.name, c.process, c.roast_level, c.tasting_notes,
		c.description, c.bag_size_grams, c.price_cents, c.currency,
		c.stock_bags, c.available,
		COALESCE(f.id::text, ''),    COALESCE(f.name, ''),
		COALESCE(f.country, ''),     COALESCE(f.region, ''),
		COALESCE(p.id::text, ''),    COALESCE(p.name, '')
	FROM coffees c
	LEFT JOIN farms f     ON c.farm_id     = f.id
	LEFT JOIN producers p ON f.producer_id = p.id`

func (s *CoffeeStore) GetByID(id string) (domain.Coffee, error) {
	row := s.db.QueryRow(selectCoffeeFields+` WHERE c.id = $1`, id)
	coffee, err := scanCoffee(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Coffee{}, domain.ErrNotFound
	}
	return coffee, err
}

func (s *CoffeeStore) List(filters store.CoffeeFilters) ([]domain.Coffee, error) {
	query := selectCoffeeFields + ` WHERE 1=1`
	args := make([]any, 0)
	n := 1

	if filters.Available != nil {
		query += fmt.Sprintf(" AND c.available = $%d", n)
		args = append(args, *filters.Available)
		n++
	}
	if filters.RoastLevel != "" {
		query += fmt.Sprintf(" AND c.roast_level = $%d", n)
		args = append(args, filters.RoastLevel)
		n++
	}
	if filters.Country != "" {
		query += fmt.Sprintf(" AND f.country = $%d", n)
		args = append(args, filters.Country)
		n++
	}
	if filters.Process != "" {
		query += fmt.Sprintf(" AND c.process = $%d", n)
		args = append(args, filters.Process)
		n++
	}

	query += " ORDER BY c.created_at DESC"

	limit := filters.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	page := filters.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list coffees: %w", err)
	}
	defer rows.Close()

	coffees := make([]domain.Coffee, 0)
	for rows.Next() {
		c, err := scanCoffee(rows)
		if err != nil {
			return nil, err
		}
		coffees = append(coffees, c)
	}
	return coffees, rows.Err()
}

func (s *CoffeeStore) Create(coffee domain.Coffee) (domain.Coffee, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.Coffee{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var producerID string
	if err := tx.QueryRow(
		`INSERT INTO producers (name) VALUES ($1) RETURNING id`,
		coffee.Producer.Name,
	).Scan(&producerID); err != nil {
		return domain.Coffee{}, fmt.Errorf("insert producer: %w", err)
	}

	var farmID string
	if err := tx.QueryRow(
		`INSERT INTO farms (name, country, region, producer_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		coffee.Farm.Name, coffee.Farm.Country, coffee.Farm.Region, producerID,
	).Scan(&farmID); err != nil {
		return domain.Coffee{}, fmt.Errorf("insert farm: %w", err)
	}

	tastingJSON, err := json.Marshal(coffee.TastingNotes)
	if err != nil {
		return domain.Coffee{}, err
	}

	var id string
	if err := tx.QueryRow(`
		INSERT INTO coffees
		    (name, process, roast_level, tasting_notes, description,
		     farm_id, bag_size_grams, price_cents, currency, stock_bags)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		coffee.Name, coffee.Process, coffee.RoastLevel, tastingJSON, coffee.Description,
		farmID, coffee.BagSizeGrams, coffee.PriceCents, coffee.Currency, coffee.StockBags,
	).Scan(&id); err != nil {
		return domain.Coffee{}, fmt.Errorf("insert coffee: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Coffee{}, err
	}
	return s.GetByID(id)
}

func (s *CoffeeStore) Update(coffee domain.Coffee) (domain.Coffee, error) {
	tastingJSON, err := json.Marshal(coffee.TastingNotes)
	if err != nil {
		return domain.Coffee{}, err
	}

	res, err := s.db.Exec(`
		UPDATE coffees
		SET name=$1, process=$2, roast_level=$3, tasting_notes=$4::jsonb,
		    description=$5, bag_size_grams=$6, price_cents=$7, currency=$8
		WHERE id=$9`,
		coffee.Name, coffee.Process, coffee.RoastLevel, tastingJSON,
		coffee.Description, coffee.BagSizeGrams, coffee.PriceCents, coffee.Currency,
		coffee.ID,
	)
	if err != nil {
		return domain.Coffee{}, fmt.Errorf("update coffee: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.Coffee{}, domain.ErrNotFound
	}
	return s.GetByID(coffee.ID)
}

func (s *CoffeeStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM coffees WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete coffee: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCoffee(s scanner) (domain.Coffee, error) {
	var c domain.Coffee
	var tastingNotesRaw []byte

	err := s.Scan(
		&c.ID, &c.Name, &c.Process, &c.RoastLevel, &tastingNotesRaw,
		&c.Description, &c.BagSizeGrams, &c.PriceCents, &c.Currency,
		&c.StockBags, &c.Available,
		&c.Farm.ID, &c.Farm.Name, &c.Farm.Country, &c.Farm.Region,
		&c.Producer.ID, &c.Producer.Name,
	)
	if err != nil {
		return domain.Coffee{}, err
	}

	if err := json.Unmarshal(tastingNotesRaw, &c.TastingNotes); err != nil {
		return domain.Coffee{}, fmt.Errorf("unmarshal tasting_notes: %w", err)
	}

	return c, nil
}
