package client

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func readDocsFixture(t *testing.T, name string) string {
	t.Helper()
	fixturePath := filepath.Join("testdata", "enable_banking_docs", name)
	content, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	return string(content)
}

func requireNoRawField(t *testing.T, value any) {
	t.Helper()
	typeOfValue := reflect.TypeOf(value)
	if typeOfValue.Kind() == reflect.Pointer {
		typeOfValue = typeOfValue.Elem()
	}
	_, found := typeOfValue.FieldByName("Raw")
	require.Falsef(t, found, "%s unexpectedly exposes Raw", typeOfValue.Name())
}
