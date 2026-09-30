package service

import (
	"coffeeproyect/internal/auth"
	"coffeeproyect/internal/domain"
	"coffeeproyect/internal/event"
	"coffeeproyect/internal/store"
	"errors"
	"fmt"
	"strings"
)

type UserService struct {
	store     store.UserStore
	jwtSecret string
	publisher event.Publisher
}

func NewUserService(s store.UserStore, jwtSecret string) *UserService {
	return &UserService{store: s, jwtSecret: jwtSecret}
}

func (s *UserService) WithPublisher(p event.Publisher) *UserService {
	s.publisher = p
	return s
}

func (s *UserService) Register(name, email, password string) (auth.TokenPair, domain.User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))

	if name == "" || email == "" || len(password) < 8 {
		return auth.TokenPair{}, domain.User{}, domain.ErrInvalidInput
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return auth.TokenPair{}, domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.store.Create(domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
	})
	if err != nil {
		return auth.TokenPair{}, domain.User{}, err
	}

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, s.jwtSecret)
	if err != nil {
		return auth.TokenPair{}, domain.User{}, fmt.Errorf("generate tokens: %w", err)
	}

	if s.publisher != nil {
		_ = s.publisher.Publish(event.TopicUserRegistered, event.UserPayload{
			UserID: user.ID,
			Email:  user.Email,
			Name:   user.Name,
		})
	}

	return tokens, user, nil
}

func (s *UserService) Login(email, password string) (auth.TokenPair, domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.store.GetByEmail(email)
	if errors.Is(err, domain.ErrNotFound) {
		return auth.TokenPair{}, domain.User{}, domain.ErrUnauthorized
	}
	if err != nil {
		return auth.TokenPair{}, domain.User{}, fmt.Errorf("get user: %w", err)
	}

	if err := auth.ComparePassword(user.PasswordHash, password); err != nil {
		return auth.TokenPair{}, domain.User{}, domain.ErrUnauthorized
	}

	tokens, err := auth.GenerateTokenPair(user.ID, user.Email, s.jwtSecret)
	if err != nil {
		return auth.TokenPair{}, domain.User{}, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, user, nil
}

func (s *UserService) RefreshToken(refreshToken string) (auth.TokenPair, error) {
	claims, err := auth.ValidateRefreshToken(refreshToken, s.jwtSecret)
	if err != nil {
		return auth.TokenPair{}, domain.ErrUnauthorized
	}

	tokens, err := auth.GenerateTokenPair(claims.UserID, claims.Email, s.jwtSecret)
	if err != nil {
		return auth.TokenPair{}, fmt.Errorf("generate tokens: %w", err)
	}
	return tokens, nil
}

func (s *UserService) GetProfile(userID string) (domain.User, []domain.Address, error) {
	user, err := s.store.GetByID(userID)
	if err != nil {
		return domain.User{}, nil, err
	}

	addresses, err := s.store.GetAddresses(userID)
	if err != nil {
		return domain.User{}, nil, err
	}

	return user, addresses, nil
}

func (s *UserService) UpdateProfile(userID, name string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.User{}, domain.ErrInvalidInput
	}

	user, err := s.store.GetByID(userID)
	if err != nil {
		return domain.User{}, err
	}

	user.Name = name
	return s.store.Update(user)
}

func (s *UserService) AddAddress(userID string, req domain.Address) (domain.Address, error) {
	req.UserID = userID
	return s.store.AddAddress(req)
}

func (s *UserService) GetAddresses(userID string) ([]domain.Address, error) {
	return s.store.GetAddresses(userID)
}

func (s *UserService) DeleteAddress(addressID, userID string) error {
	return s.store.DeleteAddress(addressID, userID)
}
