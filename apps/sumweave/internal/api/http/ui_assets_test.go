package http

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestEmbeddedUIAssets(t *testing.T) {
	t.Run("returns a usable distribution only when index exists", func(t *testing.T) {
		assert.NotNil(t, getEmbeddedUIFiles())
		assert.Nil(t, resolveEmbeddedUIFiles(fstest.MapFS{}))
		assert.Nil(
			t,
			resolveEmbeddedUIFiles(
				fstest.MapFS{"embeddedui/dist/app.js": &fstest.MapFile{Data: []byte("app")}},
			),
		)
		assert.NotNil(
			t,
			resolveEmbeddedUIFiles(
				fstest.MapFS{"embeddedui/dist/index.html": &fstest.MapFile{Data: []byte("index")}},
			),
		)
	})
}
