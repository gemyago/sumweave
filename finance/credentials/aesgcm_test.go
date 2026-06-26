package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESGCMCipher(t *testing.T) {
	fake := faker.New()
	makeKey := func() []byte {
		return []byte("0123456789abcdef0123456789abcdef")
	}

	t.Run("rejects invalid constructor inputs", func(t *testing.T) {
		_, err := NewAESGCMCipher([]byte(fake.Lorem().Word()), "fixture-key")
		require.Error(t, err)

		_, err = NewAESGCMCipher(makeKey(), "   ")
		require.Error(t, err)
	})

	t.Run("surfaces constructor dependency failures", func(t *testing.T) {
		originalAESCipher := newAESCipher
		originalNewGCM := newGCM
		t.Cleanup(func() {
			newAESCipher = originalAESCipher
			newGCM = originalNewGCM
		})

		newAESCipher = func([]byte) (cipher.Block, error) {
			return nil, assert.AnError
		}
		_, err := NewAESGCMCipher(makeKey(), fmt.Sprintf("key-%s", fake.Lorem().Word()))
		require.ErrorIs(t, err, assert.AnError)

		newAESCipher = aes.NewCipher
		newGCM = func(cipher.Block) (cipher.AEAD, error) {
			return nil, assert.AnError
		}
		_, err = NewAESGCMCipher(makeKey(), fmt.Sprintf("key-%s", fake.Lorem().Word()))
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("seals and opens plaintext", func(t *testing.T) {
		cipher, err := NewAESGCMCipher(makeKey(), fmt.Sprintf("key-%s", fake.Lorem().Word()))
		require.NoError(t, err)

		plaintext := fmt.Sprintf("secret-%s-%d", fake.Lorem().Word(), fake.Int())
		envelope, err := cipher.SealString(plaintext)
		require.NoError(t, err)
		assert.NotContains(t, envelope.Ciphertext, plaintext)

		opened, err := cipher.OpenString(envelope)
		require.NoError(t, err)
		assert.Equal(t, plaintext, opened)
	})

	t.Run("surfaces nonce read and decrypt failures", func(t *testing.T) {
		cipher, err := NewAESGCMCipher(makeKey(), fmt.Sprintf("key-%s", fake.Lorem().Word()))
		require.NoError(t, err)

		originalReader := randomReader
		randomReader = failingReader{}
		t.Cleanup(func() { randomReader = originalReader })

		_, err = cipher.SealString(fmt.Sprintf("secret-%s", fake.Lorem().Word()))
		require.Error(t, err)

		randomReader = originalReader
		envelope, err := cipher.SealString(fmt.Sprintf("secret-%s", fake.Lorem().Word()))
		require.NoError(t, err)

		_, err = cipher.OpenString(
			Envelope{Algorithm: AlgorithmAESGCM, Nonce: envelope.Nonce, Ciphertext: "***"},
		)
		require.Error(t, err)

		tampered := envelope
		tampered.Ciphertext = strings.TrimRight(envelope.Ciphertext, "=") + "AA"
		_, err = cipher.OpenString(tampered)
		require.Error(t, err)
	})

	t.Run("rejects invalid envelopes", func(t *testing.T) {
		cipher, err := NewAESGCMCipher(makeKey(), fmt.Sprintf("key-%s", fake.Lorem().Word()))
		require.NoError(t, err)

		_, err = cipher.OpenString(Envelope{Algorithm: fmt.Sprintf("bad-%s", fake.Lorem().Word())})
		require.Error(t, err)

		_, err = cipher.OpenString(
			Envelope{Algorithm: AlgorithmAESGCM, Nonce: "***", Ciphertext: "***"},
		)
		require.Error(t, err)
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}
