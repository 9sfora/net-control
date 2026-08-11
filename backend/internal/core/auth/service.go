package auth

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repository Repository
	tokens     *TokenManager
}

func NewService(repository Repository, tokens *TokenManager) *Service {
	return &Service{repository: repository, tokens: tokens}
}

func (s *Service) Login(ctx context.Context, tenant, email, password string) (string, Principal, error) {
	user, err := s.repository.FindUserForLogin(ctx, tenant, email)
	if err != nil {
		return "", Principal{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", Principal{}, ErrInvalidCredentials
	}
	principal := Principal{
		UserID: user.ID, TenantID: user.TenantID, Name: user.Name,
		Email: user.Email, Permissions: user.Permissions,
	}
	token, err := s.tokens.Issue(principal)
	if err != nil {
		return "", Principal{}, err
	}
	principal, err = s.tokens.Parse(token)
	return token, principal, err
}
