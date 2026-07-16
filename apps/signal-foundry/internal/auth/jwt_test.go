package auth

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestJWTService(t *testing.T) {
	fake := faker.New()
	makeService := func(t *testing.T, key string) *JWTService {
		t.Helper()
		service, err := NewJWTService(JWTServiceDeps{SigningKey: key, AccessTokenTTL: time.Hour})
		require.NoError(t, err)
		return service
	}

	t.Run("requires configured signing material", func(t *testing.T) {
		_, err := NewJWTService(JWTServiceDeps{AccessTokenTTL: time.Hour})
		require.ErrorContains(t, err, "JWT signing key is required")
	})
	t.Run("signs and validates HMAC tokens", func(t *testing.T) {
		service := makeService(t, fake.Lorem().Word())
		token, err := service.GenerateAccessToken(fake.UUID().V4(), fake.Internet().User())
		require.NoError(t, err)
		_, err = service.ValidateAccessToken(token)
		require.NoError(t, err)
		invalid := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)) + ".x.x"
		_, err = service.ValidateAccessToken(invalid)
		require.Error(t, err)
	})
	t.Run("rejects expired access tokens", func(t *testing.T) {
		service, err := NewJWTService(JWTServiceDeps{
			SigningKey:     fake.Lorem().Word(),
			AccessTokenTTL: -time.Second,
		})
		require.NoError(t, err)
		token, err := service.GenerateAccessToken(fake.UUID().V4(), fake.Internet().User())
		require.NoError(t, err)
		_, err = service.ValidateAccessToken(token)
		require.Error(t, err)
	})
	t.Run("wraps signing errors and rejects non-HMAC algorithms", func(t *testing.T) {
		service := makeService(t, fake.Lorem().Word())
		service.signingMethod = jwt.SigningMethodRS256
		_, err := service.GenerateAccessToken(fake.UUID().V4(), fake.Internet().User())
		require.ErrorContains(t, err, "sign access token")

		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"subject"}`))
		signature := base64.RawURLEncoding.EncodeToString([]byte("signature"))
		_, err = makeService(t, fake.Lorem().Word()).ValidateAccessToken(header + "." + payload + "." + signature)
		require.ErrorContains(t, err, "unexpected signing method")
	})
}
