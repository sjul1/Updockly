package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image/png"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/domain"
	"updockly/backend/internal/util"
	"updockly/backend/internal/vault"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type AuthService struct {
	db           *gorm.DB
	vault        *vault.Vault
	jwtPrimary   []byte
	jwtFallbacks [][]byte
}

func NewAuthService(db *gorm.DB, vault *vault.Vault, jwtPrimary string, jwtFallbacks ...string) *AuthService {
	keys := make([][]byte, 0, len(jwtFallbacks)+1)
	seen := make(map[string]struct{})
	if jwtPrimary != "" {
		keys = append(keys, []byte(jwtPrimary))
		seen[jwtPrimary] = struct{}{}
	}
	for _, fb := range jwtFallbacks {
		if fb == "" {
			continue
		}
		if _, ok := seen[fb]; ok {
			continue
		}
		seen[fb] = struct{}{}
		keys = append(keys, []byte(fb))
	}

	if len(keys) == 0 {
		keys = append(keys, []byte("dev-secret-key"))
	}

	return &AuthService{
		db:         db,
		vault:      vault,
		jwtPrimary: keys[0],
		jwtFallbacks: func() [][]byte {
			if len(keys) > 1 {
				return keys[1:]
			}
			return nil
		}(),
	}
}

func (s *AuthService) IssueToken(acc domain.Account, tokenType string, expiration time.Duration) (string, error) {
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
	return token.SignedString(s.jwtPrimary)
}

func (s *AuthService) IssueRefreshToken(acc *domain.Account, ttl time.Duration) (string, error) {
	if s.db == nil {
		return "", errors.New("database not configured")
	}
	token := util.RandomString(64)
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	expiry := time.Now().Add(ttl)
	acc.RefreshTokenHash = hashHex
	acc.RefreshTokenExpiry = &expiry
	if err := s.db.Save(acc).Error; err != nil {
		return "", err
	}
	return token, nil
}

func (s *AuthService) ValidateRefreshToken(token string) (*domain.Account, error) {
	if token == "" {
		return nil, errors.New("missing token")
	}
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	var account domain.Account
	silent := s.db.Session(&gorm.Session{Logger: logger.Discard})
	if err := silent.Where("refresh_token_hash = ?", hashHex).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid token")
		}
		return nil, err
	}
	if account.RefreshTokenExpiry != nil && time.Now().After(*account.RefreshTokenExpiry) {
		return nil, errors.New("token expired")
	}
	return &account, nil
}

func (s *AuthService) ClearRefreshToken(username string) error {
	if s.db == nil {
		return nil
	}
	return s.db.Model(&domain.Account{}).Where("username = ?", username).Updates(map[string]interface{}{
		"refresh_token_hash":   "",
		"refresh_token_expiry": nil,
	}).Error
}

func (s *AuthService) VerifyToken(tokenString string) (*TokenClaims, error) {
	parseWithKey := func(key []byte) (*TokenClaims, error) {
		token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return key, nil
		})
		if err != nil {
			return nil, err
		}
		if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
			return claims, nil
		}
		return nil, errors.New("invalid token")
	}

	if claims, err := parseWithKey(s.jwtPrimary); err == nil {
		return claims, nil
	}

	for _, fb := range s.jwtFallbacks {
		if claims, err := parseWithKey(fb); err == nil {
			return claims, nil
		}
	}

	return nil, errors.New("invalid token")
}

func (s *AuthService) Authenticate(username, password string) (*domain.Account, error) {
	var account domain.Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !checkPassword(account.PasswordHash, password) {
		return nil, errors.New("invalid credentials")
	}

	return &account, nil
}

func (s *AuthService) GetAccount(username string) (*domain.Account, error) {
	var account domain.Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *AuthService) UpdateAccount(username, name, email, currentPassword, newPassword string) (*domain.Account, error) {
	var account domain.Account
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
	if err := s.db.Model(&domain.Account{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func hashCodes(codes []string) []string {
	hashed := make([]string, len(codes))
	for i, c := range codes {
		hashed[i] = hashSecret(c)
	}
	return hashed
}

func verifyRecoveryCode(code string, hashedCodes []string) (int, bool) {
	for i, h := range hashedCodes {
		if checkPassword(h, code) {
			return i, true
		}
	}
	return -1, false
}

func (s *AuthService) CreateAdmin(username, email, password, name, totpSecret string) ([]string, error) {
	cipher, err := s.vault.Encrypt(totpSecret)
	if err != nil {
		return nil, err
	}

	account := domain.Account{
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
		codes[i] = util.RandomString(10)
	}
	account.RecoveryCodes = hashCodes(codes)

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
	var account domain.Account
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
	var account domain.Account
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
		codes[i] = util.RandomString(10)
	}
	account.RecoveryCodes = hashCodes(codes)

	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *AuthService) Disable2FA(username, code, password string) error {
	var account domain.Account
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

	valid := false
	if err == nil {
		valid = totp.Validate(code, secret)
	}

	if !valid {
		// Check recovery codes
		_, found := verifyRecoveryCode(code, account.RecoveryCodes)
		if found {
			valid = true
		}
	}

	if !valid {
		if err != nil {
			return err
		}
		return errors.New("invalid 2fa code")
	}

	account.TwoFactorEnabled = false
	return s.db.Save(&account).Error
}

func (s *AuthService) InitiateReset2FA(username, recoveryCode, password string) (string, string, string, error) {
	var account domain.Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return "", "", "", err
	}

	if !account.TwoFactorEnabled {
		return "", "", "", errors.New("2fa not enabled")
	}

	if !checkPassword(account.PasswordHash, password) {
		return "", "", "", errors.New("invalid password")
	}

	// Verify recovery code
	_, valid := verifyRecoveryCode(recoveryCode, account.RecoveryCodes)

	if !valid {
		return "", "", "", errors.New("invalid recovery code")
	}

	// Generate new TOTP
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Updockly",
		AccountName: username,
	})
	if err != nil {
		return "", "", "", err
	}

	secret := key.Secret()
	cipher, err := s.vault.Encrypt(secret)
	if err != nil {
		return "", "", "", err
	}

	// Temporarily disable 2FA until verified in next step
	account.TwoFactorSecret = cipher
	account.TwoFactorEnabled = false
	account.RecoveryCodes = []string{}

	if err := s.db.Save(&account).Error; err != nil {
		return "", "", "", err
	}

	var buf bytes.Buffer
	img, err := key.Image(256, 256)
	if err != nil {
		return "", "", "", err
	}
	if err := png.Encode(&buf, img); err != nil {
		return "", "", "", err
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	token, err := s.IssueToken(account, "reset-2fa-verify", 15*time.Minute)
	if err != nil {
		return "", "", "", err
	}

	return secret, dataURI, token, nil
}

func (s *AuthService) FinalizeReset2FA(tempToken, code string) ([]string, error) {
	claims, err := s.VerifyToken(tempToken)
	if err != nil {
		return nil, err
	}
	if claims.Type != "reset-2fa-verify" {
		return nil, errors.New("invalid token type")
	}

	var account domain.Account
	if err := s.db.Where("username = ?", claims.Subject).First(&account).Error; err != nil {
		return nil, err
	}

	secret, err := s.vault.Decrypt(account.TwoFactorSecret)
	if err != nil {
		return nil, err
	}

	if !totp.Validate(code, secret) {
		return nil, errors.New("invalid 2fa code")
	}

	// Generate new recovery codes
	newCodes := make([]string, 10)
	for i := 0; i < 10; i++ {
		newCodes[i] = util.RandomString(10)
	}

	account.RecoveryCodes = hashCodes(newCodes)
	account.TwoFactorEnabled = true

	if err := s.db.Save(&account).Error; err != nil {
		return nil, err
	}

	return newCodes, nil
}

func (s *AuthService) Validate2FA(username, code string) (bool, error) {
	var account domain.Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return false, err
	}

	secret, err := s.vault.Decrypt(account.TwoFactorSecret)
	// If decryption fails, we treat the TOTP as invalid but continue to check recovery codes.
	// Only return error if the code is NOT a valid recovery code either.

	var valid bool
	if err == nil {
		valid = totp.Validate(code, secret)
	}

	if !valid {
		// Check recovery codes
		idx, found := verifyRecoveryCode(code, account.RecoveryCodes)
		if found {
			// Remove used code
			account.RecoveryCodes = append(account.RecoveryCodes[:idx], account.RecoveryCodes[idx+1:]...)
			if err := s.db.Save(&account).Error; err != nil {
				return false, err
			}
			return true, nil
		}
	}

	// If we failed TOTP validation due to decryption error AND it wasn't a recovery code,
	// then we should probably return the decryption error if it existed, or just false.
	// But to maintain existing behavior for normal invalid codes, we return valid (false) and nil error if decryption worked but code was wrong.
	// If decryption failed, and it wasn't a recovery code, we return the decryption error so logs show why.
	if err != nil {
		return false, err
	}

	return valid, nil
}

func (s *AuthService) ResetPasswordWithRecoveryCode(username, code, newPassword string) error {
	var account domain.Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return errors.New("user not found")
	}

	idx, found := verifyRecoveryCode(code, account.RecoveryCodes)

	if !found {
		return errors.New("invalid recovery code")
	}

	// Remove used code
	account.RecoveryCodes = append(account.RecoveryCodes[:idx], account.RecoveryCodes[idx+1:]...)
	account.PasswordHash = hashSecret(newPassword)

	return s.db.Save(&account).Error
}

func (s *AuthService) RegenerateRecoveryCodes(username, password string) ([]string, error) {
	var account domain.Account
	if err := s.db.Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}

	if !account.TwoFactorEnabled {
		return nil, errors.New("2fa not enabled")
	}

	if !checkPassword(account.PasswordHash, password) {
		return nil, errors.New("invalid password")
	}

	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		codes[i] = util.RandomString(10)
	}
	account.RecoveryCodes = hashCodes(codes)

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
func (s *AuthService) FindAccountForSSO(identifier string) (*domain.Account, error) {
	var account domain.Account
	// Case-insensitive check
	err := s.db.Session(&gorm.Session{Logger: logger.Discard}).Where("LOWER(username) = ?", strings.ToLower(identifier)).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *AuthService) GeneratePasswordResetToken(email string) (string, *domain.Account, error) {
	var account domain.Account
	if err := s.db.Where("email = ?", email).First(&account).Error; err != nil {
		return "", nil, errors.New("user not found")
	}

	token := util.RandomString(32)
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	expiry := time.Now().Add(1 * time.Hour)
	account.ResetToken = ""
	account.ResetTokenHash = hashHex
	account.ResetTokenExpiry = &expiry
	if err := s.db.Model(&domain.Account{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
		"reset_token":        "",
		"reset_token_hash":   hashHex,
		"reset_token_expiry": expiry,
	}).Error; err != nil {
		return "", nil, err
	}

	return token, &account, nil
}

func (s *AuthService) ResetPasswordWithToken(token, newPassword string) error {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	var account domain.Account
	if err := s.db.Where("reset_token_hash = ?", hashHex).First(&account).Error; err != nil {
		return errors.New("invalid token")
	}

	if account.ResetTokenExpiry != nil && time.Now().After(*account.ResetTokenExpiry) {
		return errors.New("token expired")
	}

	account.PasswordHash = hashSecret(newPassword)
	account.ResetToken = ""
	account.ResetTokenHash = ""
	account.ResetTokenExpiry = nil

	return s.db.Save(&account).Error
}
