package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"villageconnect/internal/models"
)

const authTestSecret = "unit-test-secret-with-more-than-thirty-two-characters"

func TestSessionTokenClaimsAndValidation(t *testing.T) {
	user := models.User{ID: "user-1", Email: "user@example.com", Role: models.RoleCustomer}
	rawToken, claims, err := GenerateSessionToken(authTestSecret, user, time.Hour)
	if err != nil {
		t.Fatalf("GenerateSessionToken returned error: %v", err)
	}
	if claims.ID == "" || claims.Subject != user.ID || claims.Role != string(user.Role) {
		t.Fatalf("unexpected session claims: %+v", claims)
	}
	parsed, err := ValidateToken(authTestSecret, "Bearer "+rawToken)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
	if parsed.ID != claims.ID || parsed.UserID != user.ID {
		t.Fatalf("parsed claims = %+v, want session %q for user %q", parsed, claims.ID, user.ID)
	}
	if _, err := ValidateToken(authTestSecret, "Bearer "+rawToken+"tampered"); err == nil {
		t.Fatal("tampered token was accepted")
	}
	if _, err := ValidateToken(authTestSecret, rawToken); err == nil {
		t.Fatal("token without Bearer scheme was accepted")
	}
}

func TestValidateTokenRejectsWrongAlgorithmAndAudience(t *testing.T) {
	user := models.User{ID: "user-1", Role: models.RoleCustomer}
	tests := []struct {
		name   string
		method jwt.SigningMethod
		issuer string
		aud    jwt.ClaimStrings
	}{
		{"wrong algorithm", jwt.SigningMethodHS384, "villageconnect", jwt.ClaimStrings{"villageconnect-api"}},
		{"wrong audience", jwt.SigningMethodHS256, "villageconnect", jwt.ClaimStrings{"other-api"}},
		{"wrong issuer", jwt.SigningMethodHS256, "other-service", jwt.ClaimStrings{"villageconnect-api"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now()
			claims := Claims{
				UserID: user.ID,
				Role:   string(user.Role),
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer: test.issuer, Subject: user.ID, Audience: test.aud, ID: "session-1",
					IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
			}
			token := jwt.NewWithClaims(test.method, claims)
			raw, err := token.SignedString([]byte(authTestSecret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateToken(authTestSecret, "Bearer "+raw); err == nil {
				t.Fatal("invalid token configuration was accepted")
			}
		})
	}
}

func TestGenerateTokenRejectsWeakSecretsAndInvalidLifetime(t *testing.T) {
	user := models.User{ID: "user-1", Role: models.RoleCustomer}
	for _, test := range []struct {
		secret   string
		lifetime time.Duration
	}{
		{"short", time.Hour},
		{authTestSecret, 0},
		{authTestSecret, 25 * time.Hour},
	} {
		if _, _, err := GenerateSessionToken(test.secret, user, test.lifetime); err == nil {
			t.Fatal("invalid token configuration was accepted")
		}
	}
}
