package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type AuthService struct {
	db        *gorm.DB
	vault     *Vault
	jwtSecret []byte
}

func NewAuthService(db *gorm.DB, vault *Vault, jwtSecret string) *AuthService {
	return &AuthService{
		db:        db,
		vault:     vault,
		jwtSecret: []byte(jwtSecret),
	}
}

func (s *AuthService) IssueToken(acc Account, tokenType string, expiration time.Duration) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   acc.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiration)),
		},
		Role: acc.Role,
		Name: acc.Name,
		Type: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) VerifyToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func (s *AuthService) Authenticate(username, password string) (*Account, error) {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !checkPassword(account.PasswordHash, password) {
		return nil, errors.New("invalid credentials")
	}

	return &account, nil
}

func (s *AuthService) GetAccount(username string) (*Account, error) {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *AuthService) UpdateAccount(username, name, email, currentPassword, newPassword string) (*Account, error) {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}

	if newPassword != "" {
		if currentPassword == "" {
			return nil, errors.New("current password required to change password")
		}
		if !checkPassword(account.PasswordHash, currentPassword) {
			return nil, errors.New("invalid current password")
		}
		account.PasswordHash = hashSecret(newPassword)
	}

	if strings.TrimSpace(name) != "" {
		account.Name = name
	}
	account.Email = strings.TrimSpace(email)

	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *AuthService) AccountExists() (bool, error) {
	if s.db == nil {
		return false, nil
	}
	var count int64
	if err := s.db.Model(&Account{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *AuthService) CreateAdmin(username, email, password, name, totpSecret string) ([]string, error) {
	cipher, err := s.vault.Encrypt(totpSecret)
	if err != nil {
		return nil, err
	}

	account := Account{
		Username:         username,
		Email:            email,
		Name:             name,
		PasswordHash:     hashSecret(password),
		Role:             "admin",
		TwoFactorSecret:  cipher,
		TwoFactorEnabled: true,
	}

	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		codes[i] = randomString(10)
	}
	account.RecoveryCodes = codes

	if err := s.db.Create(&account).Error; err != nil {
		return nil, err
	}

	return codes, nil
}

func (s *AuthService) Generate2FA(username string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Updockly",
		AccountName: username,
	})
	if err != nil {
		return "", "", err
	}

	secret := key.Secret()
	cipher, err := s.vault.Encrypt(secret)
	if err != nil {
		return "", "", err
	}

	// Save secret temporarily (or update user with pending secret? The original logic updated the user immediately)
	// Original logic: updated user with secret.
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return "", "", err
	}
	account.TwoFactorSecret = cipher
	if err := s.db.Save(&account).Error; err != nil {
		return "", "", err
	}

	var buf bytes.Buffer
	img, err := key.Image(256, 256)
	if err != nil {
		return "", "", err
	}
	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	return secret, dataURI, nil
}

func (s *AuthService) Enable2FA(username, code string) ([]string, error) {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}

	if account.TwoFactorSecret == "" {
		return nil, errors.New("2fa not set up")
	}

	secret, err := s.vault.Decrypt(account.TwoFactorSecret)
	if err != nil {
		return nil, err
	}

	if !totp.Validate(code, secret) {
		return nil, errors.New("invalid 2fa code")
	}

	account.TwoFactorEnabled = true
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		codes[i] = randomString(10)
	}
	account.RecoveryCodes = codes

	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *AuthService) Disable2FA(username, code, password string) error {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return err
	}

	if !account.TwoFactorEnabled {
		return errors.New("2fa not enabled")
	}

	if !checkPassword(account.PasswordHash, password) {
		return errors.New("invalid password")
	}

	secret, err := s.vault.Decrypt(account.TwoFactorSecret)
	if err != nil {
		return err
	}

	if !totp.Validate(code, secret) {
		return errors.New("invalid 2fa code")
	}

	account.TwoFactorEnabled = false
	return s.db.Save(&account).Error
}

func (s *AuthService) Validate2FA(username, code string) (bool, error) {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return false, err
	}

	secret, err := s.vault.Decrypt(account.TwoFactorSecret)
	if err != nil {
		return false, err
	}

	valid := totp.Validate(code, secret)
	if !valid {
		// Check recovery codes
		found := -1
		for i, rc := range account.RecoveryCodes {
			if rc == code {
				found = i
				break
			}
		}
		if found != -1 {
			// Remove used code
			account.RecoveryCodes = append(account.RecoveryCodes[:found], account.RecoveryCodes[found+1:]...)
			if err := s.db.Save(&account).Error; err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return valid, nil
}

func (s *AuthService) ResetPasswordWithRecoveryCode(username, code, newPassword string) error {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return errors.New("user not found")
	}

	found := -1
	for i, rc := range account.RecoveryCodes {
		if rc == code {
			found = i
			break
		}
	}

	if found == -1 {
		return errors.New("invalid recovery code")
	}

	// Remove used code
	account.RecoveryCodes = append(account.RecoveryCodes[:found], account.RecoveryCodes[found+1:]...)
	account.PasswordHash = hashSecret(newPassword)

	return s.db.Save(&account).Error
}

func (s *AuthService) RegenerateRecoveryCodes(username string) ([]string, error) {
	var account Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}

	if !account.TwoFactorEnabled {
		return nil, errors.New("2fa not enabled")
	}

	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		codes[i] = randomString(10)
	}
	account.RecoveryCodes = codes

	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *AuthService) GenerateSetupOTP() (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Updockly",
		AccountName: "admin",
	})
	if err != nil {
		return "", "", err
	}

	var buf bytes.Buffer
	img, err := key.Image(256, 256)
	if err != nil {
		return "", "", err
	}
	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	return key.Secret(), dataURI, nil
}

// SsoLogin helper to find or create user? Currently the handler just checks existence.
// We can add `GetAccountByEmailOrUsername` kind of logic.
func (s *AuthService) FindAccountForSSO(identifier string) (*Account, error) {
	var account Account
	// Case-insensitive check
	err := s.db.Session(&gorm.Session{Logger: logger.Discard}).Where("LOWER(username) = ?", strings.ToLower(identifier)).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *AuthService) GeneratePasswordResetToken(email string) (string, *Account, error) {
	var account Account
	if err := s.db.Where("email = ?", email).First(&account).Error; err != nil {
		return "", nil, errors.New("user not found")
	}

	token := randomString(32)
	expiry := time.Now().Add(1 * time.Hour)
	account.ResetToken = token
	account.ResetTokenExpiry = &expiry

	if err := s.db.Save(&account).Error; err != nil {
		return "", nil, err
	}

	return token, &account, nil
}

func (s *AuthService) ResetPasswordWithToken(token, newPassword string) error {
	var account Account
	if err := s.db.Where("reset_token = ?", token).First(&account).Error; err != nil {
		return errors.New("invalid token")
	}

	if account.ResetTokenExpiry != nil && time.Now().After(*account.ResetTokenExpiry) {
		return errors.New("token expired")
	}

	account.PasswordHash = hashSecret(newPassword)
	account.ResetToken = ""
	account.ResetTokenExpiry = nil

	return s.db.Save(&account).Error
}
