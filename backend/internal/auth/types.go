package auth

import (
	"updockly/backend/internal/domain"
	"updockly/backend/internal/vault"
)

type (
	Account = domain.Account
)

func NewVault(primary, secret, previous string) *vault.Vault {
	return vault.NewVault(primary, secret, previous)
}
