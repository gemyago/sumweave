package auth

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/gemyago/sumweave/apps/sumweave/internal/system/ident"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenService(t *testing.T) {
	fake := faker.New()
	makeDeps := func(t *testing.T, random []byte) AccessTokenServiceDeps {
		t.Helper()
		location := time.FixedZone(fake.Lorem().Word(), 2*60*60)
		return AccessTokenServiceDeps{
			Store:        newMockaccessTokenStore(t),
			Users:        newMockaccessTokenUserReader(t),
			IDGen:        ident.NewMockGenerator(),
			Clock:        func() time.Time { return time.Date(2026, time.September, 15, 10, 11, 12, 123456789, location) },
			RandomReader: bytes.NewReader(random),
			Logger:       telemetry.RootTestLogger(),
		}
	}
	makeUserID := func() string { return ident.NewDefaultGenerator().MustNewV7().String() }
	makeToken := func(userID string, secret []byte, now time.Time) AccessToken {
		id := ident.NewDefaultGenerator().MustNewV7()
		digest := sha256.Sum256(secret)
		return AccessToken{
			ID:         id.String(),
			UserID:     userID,
			Name:       "token-" + fake.UUID().V4(),
			Permission: AccessTokenPermissionReadOnly,
			SecretHash: digest[:],
			CreatedAt:  now,
		}
	}
	makeSecret := func(value byte) []byte { return bytes.Repeat([]byte{value}, accessTokenSecretBytes) }

	t.Run("issues one-time prefixed 32-byte secrets and stores only their digest", func(t *testing.T) {
		secret := makeSecret(0xff)
		deps := makeDeps(t, secret)
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID := makeUserID()
		name := "  " + fake.Lorem().Word() + "  "
		user := &User{ID: userID, Username: fake.Internet().User()}
		deps.Users.(*mockaccessTokenUserReader).EXPECT().GetByID(t.Context(), userID).Return(user, nil)
		deps.Store.(*mockaccessTokenStore).EXPECT().Create(t.Context(), mock.MatchedBy(func(token AccessToken) bool {
			digest := sha256.Sum256(secret)
			return token.UserID == userID &&
				token.Name == name[2:len(name)-2] &&
				token.Permission == AccessTokenPermissionReadWrite &&
				bytes.Equal(token.SecretHash, digest[:]) && len(token.SecretHash) == sha256.Size
		})).Return(nil)

		issued, err := service.Create(t.Context(), CreateAccessTokenParams{
			UserID: userID, Name: name, Permission: AccessTokenPermissionReadWrite,
		})
		require.NoError(t, err)
		require.Contains(t, issued.APIToken, accessTokenPrefix)
		tokenID, parsedSecret, err := parseAccessToken(issued.APIToken)
		require.NoError(t, err)
		require.Equal(t, issued.ID, tokenID)
		require.Equal(t, secret, parsedSecret)
		require.Contains(t, issued.APIToken, "_")
		require.Equal(t, AccessTokenStatusActive, issued.Status)
	})

	t.Run("parses base64url secrets containing underscores and rejects malformed values", func(t *testing.T) {
		id := ident.NewDefaultGenerator().MustNewV7().String()
		value := formatAccessToken(id, makeSecret(0xff))
		_, secret, err := parseAccessToken(value)
		require.NoError(t, err)
		require.Equal(t, makeSecret(0xff), secret)
		for _, malformed := range []string{
			fake.Lorem().Text(30), accessTokenPrefix + fake.UUID().V4() + "_" + value[len(value)-4:],
			accessTokenPrefix + id + "_" + fake.Lorem().Text(10),
		} {
			_, _, parseErr := parseAccessToken(malformed)
			require.Error(t, parseErr)
		}
	})

	t.Run("validates stored digests and returns uniform invalid outcomes", func(t *testing.T) {
		secret := makeSecret(0xfe)
		deps := makeDeps(t, secret)
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID := makeUserID()
		now := service.now()
		valid := makeToken(userID, secret, now)
		validValue := formatAccessToken(valid.ID, secret)
		deps.Store.(*mockaccessTokenStore).EXPECT().GetByID(t.Context(), valid.ID).Return(&valid, nil).Once()
		metadata, err := service.Validate(t.Context(), validValue)
		require.NoError(t, err)
		require.Equal(t, valid.ID, metadata.ID)

		mismatched := makeToken(userID, makeSecret(0xfd), now)
		mismatched.ID = valid.ID
		expiredTime := now.Add(-time.Second)
		expired := valid
		expired.ExpiresAt = &expiredTime
		revokedTime := now.Add(-time.Second)
		revoked := valid
		revoked.RevokedAt = &revokedTime
		invalidPermission := valid
		invalidPermission.Permission = AccessTokenPermission(fake.Lorem().Word())
		for _, token := range []AccessToken{mismatched, expired, revoked, invalidPermission} {
			deps.Store.(*mockaccessTokenStore).EXPECT().GetByID(t.Context(), valid.ID).Return(&token, nil).Once()
			_, validateErr := service.Validate(t.Context(), validValue)
			require.ErrorIs(t, validateErr, ErrInvalidAccessToken)
		}
		deps.Store.(*mockaccessTokenStore).EXPECT().
			GetByID(t.Context(), valid.ID).
			Return(nil, ErrAccessTokenNotFound).
			Once()
		_, err = service.Validate(t.Context(), validValue)
		require.ErrorIs(t, err, ErrInvalidAccessToken)
		_, err = service.Validate(t.Context(), fake.Lorem().Text(20))
		require.ErrorIs(t, err, ErrInvalidAccessToken)
	})

	t.Run("validates names and expiries before persistence", func(t *testing.T) {
		deps := makeDeps(t, makeSecret(1))
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID := makeUserID()
		past := service.now().Add(-time.Second)
		for _, params := range []CreateAccessTokenParams{
			{UserID: userID, Name: "   ", Permission: AccessTokenPermissionReadOnly},
			{UserID: userID, Name: string(bytes.Repeat([]byte("x"), 101)), Permission: AccessTokenPermissionReadOnly},
			{UserID: userID, Name: fake.Lorem().Word(), Permission: AccessTokenPermission(fake.Lorem().Word())},
			{UserID: userID, Name: fake.Lorem().Word(), Permission: AccessTokenPermissionReadOnly, ExpiresAt: &past},
		} {
			_, inputErr := service.Create(t.Context(), params)
			require.ErrorIs(t, inputErr, ErrInvalidAccessTokenInput)
		}
	})

	t.Run("lists safe statuses and revokes only owned token IDs idempotently", func(t *testing.T) {
		deps := makeDeps(t, makeSecret(1))
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID := makeUserID()
		now := service.now()
		expiredAt := now.Add(-time.Second)
		revokedAt := now.Add(-time.Second)
		active := makeToken(userID, makeSecret(2), now)
		expired := makeToken(userID, makeSecret(3), now)
		expired.ExpiresAt = &expiredAt
		revoked := makeToken(userID, makeSecret(4), now)
		revoked.RevokedAt = &revokedAt
		deps.Store.(*mockaccessTokenStore).EXPECT().
			ListByUserID(t.Context(), userID).
			Return([]AccessToken{active, expired, revoked}, nil)
		listed, err := service.List(t.Context(), userID)
		require.NoError(t, err)
		require.Equal(
			t,
			[]AccessTokenStatus{AccessTokenStatusActive, AccessTokenStatusExpired, AccessTokenStatusRevoked},
			[]AccessTokenStatus{listed[0].Status, listed[1].Status, listed[2].Status},
		)
		deps.Store.(*mockaccessTokenStore).EXPECT().
			Revoke(t.Context(), userID, active.ID, service.now()).
			Return(nil).
			Twice()
		require.NoError(t, service.Revoke(t.Context(), userID, active.ID))
		require.NoError(t, service.Revoke(t.Context(), userID, active.ID))
		otherID := makeUserID()
		deps.Store.(*mockaccessTokenStore).EXPECT().
			Revoke(t.Context(), userID, otherID, service.now()).
			Return(ErrAccessTokenNotFound)
		require.ErrorIs(t, service.Revoke(t.Context(), userID, otherID), ErrAccessTokenNotFound)
	})

	t.Run("rotates transactionally with injected IDs and reports concurrent conflicts", func(t *testing.T) {
		secret := makeSecret(0xfc)
		deps := makeDeps(t, append(secret, secret...))
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID, oldID := makeUserID(), ident.NewDefaultGenerator().MustNewV7().String()
		newID := ident.MockGeneratorNextGenerated(deps.IDGen)
		now := service.now()
		replacement := makeToken(userID, secret, now)
		replacement.ID = newID.String()
		expectedHash := sha256.Sum256(secret)
		deps.Store.(*mockaccessTokenStore).EXPECT().
			Rotate(t.Context(), mock.MatchedBy(func(params RotateAccessTokenParams) bool {
				return params.UserID == userID &&
					params.TokenID == oldID &&
					params.ReplacementTokenID == newID.String() &&
					bytes.Equal(params.SecretHash, expectedHash[:])
			})).
			Return(&replacement, nil).
			Once()
		issued, err := service.Rotate(t.Context(), RotateAccessTokenRequest{UserID: userID, TokenID: oldID})
		require.NoError(t, err)
		require.Equal(t, replacement.ID, issued.ID)
		deps.Store.(*mockaccessTokenStore).EXPECT().
			Rotate(t.Context(), mock.Anything).
			Return(nil, ErrAccessTokenConflict).
			Once()
		_, err = service.Rotate(t.Context(), RotateAccessTokenRequest{UserID: userID, TokenID: oldID})
		require.ErrorIs(t, err, ErrAccessTokenConflict)
	})

	t.Run("requires every constructor dependency and propagates random reader exhaustion", func(t *testing.T) {
		deps := makeDeps(t, nil)
		for _, clear := range []func(*AccessTokenServiceDeps){
			func(value *AccessTokenServiceDeps) { value.Store = nil },
			func(value *AccessTokenServiceDeps) { value.Users = nil },
			func(value *AccessTokenServiceDeps) { value.IDGen = nil },
			func(value *AccessTokenServiceDeps) { value.Clock = nil },
			func(value *AccessTokenServiceDeps) { value.RandomReader = nil },
			func(value *AccessTokenServiceDeps) { value.Logger = nil },
		} {
			candidate := deps
			clear(&candidate)
			_, err := NewAccessTokenService(candidate)
			require.Error(t, err)
		}
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID := makeUserID()
		deps.Users.(*mockaccessTokenUserReader).EXPECT().GetByID(t.Context(), userID).Return(&User{ID: userID}, nil)
		_, err = service.Create(t.Context(), CreateAccessTokenParams{
			UserID: userID, Name: fake.Lorem().Word(), Permission: AccessTokenPermissionReadOnly,
		})
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrInvalidAccessTokenInput)
	})

	t.Run("propagates lifecycle persistence errors without issuing a token", func(t *testing.T) {
		secret := makeSecret(0xfb)
		deps := makeDeps(t, secret)
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		userID := makeUserID()
		deps.Users.(*mockaccessTokenUserReader).EXPECT().
			GetByID(t.Context(), userID).
			Return(&User{ID: userID}, nil)
		deps.Store.(*mockaccessTokenStore).EXPECT().
			Create(t.Context(), mock.Anything).
			Return(errors.New(fake.Lorem().Sentence(2)))
		_, err = service.Create(t.Context(), CreateAccessTokenParams{
			UserID: userID, Name: fake.Lorem().Word(), Permission: AccessTokenPermissionReadOnly,
		})
		require.Error(t, err)
		deps.Store.(*mockaccessTokenStore).EXPECT().
			ListByUserID(t.Context(), userID).
			Return(nil, errors.New(fake.Lorem().Sentence(2)))
		_, err = service.List(t.Context(), userID)
		require.Error(t, err)
		tokenID := makeUserID()
		deps.Store.(*mockaccessTokenStore).EXPECT().
			Revoke(t.Context(), userID, tokenID, service.now()).
			Return(errors.New(fake.Lorem().Sentence(2)))
		require.Error(t, service.Revoke(t.Context(), userID, tokenID))
		past := service.now().Add(-time.Second)
		_, err = service.Rotate(t.Context(), RotateAccessTokenRequest{
			UserID: userID, TokenID: tokenID, ExpiresAt: &past,
		})
		require.ErrorIs(t, err, ErrInvalidAccessTokenInput)
	})

	t.Run("rejects malformed owners and propagates lookup and validation errors", func(t *testing.T) {
		deps := makeDeps(t, makeSecret(5))
		service, err := NewAccessTokenService(deps)
		require.NoError(t, err)
		validUserID := makeUserID()

		_, err = service.Create(t.Context(), CreateAccessTokenParams{
			UserID: "not-a-uuid", Name: fake.Lorem().Word(), Permission: AccessTokenPermissionReadOnly,
		})
		require.ErrorIs(t, err, ErrInvalidAccessTokenInput)
		_, err = service.List(t.Context(), "not-a-uuid")
		require.ErrorIs(t, err, ErrInvalidAccessTokenInput)
		require.ErrorIs(t, service.Revoke(t.Context(), "not-a-uuid", validUserID), ErrAccessTokenNotFound)
		require.ErrorIs(t, service.Revoke(t.Context(), validUserID, "not-a-uuid"), ErrAccessTokenNotFound)

		deps.Users.(*mockaccessTokenUserReader).EXPECT().
			GetByID(t.Context(), validUserID).
			Return(nil, ErrUserNotFound).Once()
		_, err = service.Create(t.Context(), CreateAccessTokenParams{
			UserID: validUserID, Name: fake.Lorem().Word(), Permission: AccessTokenPermissionReadOnly,
		})
		require.ErrorIs(t, err, ErrUserNotFound)

		unknownErr := errors.New(fake.Lorem().Sentence(2))
		deps.Users.(*mockaccessTokenUserReader).EXPECT().
			GetByID(t.Context(), validUserID).
			Return(nil, unknownErr).Once()
		_, err = service.Create(t.Context(), CreateAccessTokenParams{
			UserID: validUserID, Name: fake.Lorem().Word(), Permission: AccessTokenPermissionReadOnly,
		})
		require.ErrorIs(t, err, unknownErr)

		presented := makeToken(validUserID, makeSecret(6), service.now())
		deps.Store.(*mockaccessTokenStore).EXPECT().
			GetByID(t.Context(), presented.ID).
			Return(nil, unknownErr).Once()
		_, err = service.Validate(t.Context(), formatAccessToken(presented.ID, makeSecret(6)))
		require.ErrorIs(t, err, unknownErr)
	})
}
