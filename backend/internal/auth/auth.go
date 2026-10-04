package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"villageconnect/internal/models"
)

// Claims describes the authenticated session payload embedded in JWTs.
type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, user models.User) (string, error) {
	token, _, err := GenerateSessionToken(secret, user, 24*time.Hour)
	return token, err
}

func GenerateSessionToken(secret string, user models.User, lifetime time.Duration) (string, *Claims, error) {
	if len(secret) < 32 || strings.TrimSpace(user.ID) == "" || !validRole(string(user.Role)) ||
		lifetime <= 0 || lifetime > 24*time.Hour {
		return "", nil, errors.New("invalid token configuration")
	}
	jti, err := randomID()
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	claims := Claims{
		UserID: user.ID,
		Role:   string(user.Role),
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "villageconnect",
			Subject:   user.ID,
			Audience:  jwt.ClaimStrings{"villageconnect-api"},
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	rawToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", nil, err
	}
	return rawToken, &claims, nil
}

func ValidateToken(secret, rawToken string) (*Claims, error) {
	if len(secret) < 32 {
		return nil, errors.New("invalid JWT secret configuration")
	}
	parts := strings.Fields(strings.TrimSpace(rawToken))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return nil, errors.New("missing bearer token")
	}

	token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithIssuer("villageconnect"), jwt.WithAudience("villageconnect-api"),
		jwt.WithExpirationRequired(), jwt.WithIssuedAt(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.UserID == "" || claims.Subject != claims.UserID || claims.ID == "" ||
		!validRole(claims.Role) || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

func validRole(role string) bool {
	switch models.UserRole(role) {
	case models.RoleCustomer, models.RoleSeller, models.RoleDeliveryAgent, models.RoleAdmin:
		return true
	default:
		return false
	}
}

func randomID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
