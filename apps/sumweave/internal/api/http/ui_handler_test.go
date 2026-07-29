package http

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gemyago/sumweave/apps/sumweave/internal/api/http/server"
	"github.com/gemyago/sumweave/apps/sumweave/internal/telemetry"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

type subErrorFS struct{}

func (subErrorFS) Open(string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (subErrorFS) Sub(string) (fs.FS, error) {
	return nil, fs.ErrNotExist
}

func TestUIHandler(t *testing.T) {
	fake := faker.New()

	makeRequest := func(t *testing.T, handler http.Handler, method string, target string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, target, http.NoBody)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	t.Run("serves files and SPA fallback", func(t *testing.T) {
		indexContent := fake.Lorem().Sentence(6)
		assetPath := "assets/" + fake.UUID().V4() + ".js"
		assetContent := fake.Lorem().Sentence(4)
		nestedIndexContent := fake.Lorem().Sentence(3)
		handler := newUIHandler(fstest.MapFS{
			"index.html":                {Data: []byte(indexContent)},
			assetPath:                   {Data: []byte(assetContent)},
			"favicon.svg":               {Data: []byte(fake.Lorem().Sentence(2))},
			"nested/index.html":         {Data: []byte(nestedIndexContent)},
			"images/logo-marketing.svg": {Data: []byte(fake.Lorem().Sentence(2))},
		})

		for _, testCase := range []struct {
			name       string
			method     string
			target     string
			wantStatus int
			wantBody   string
		}{
			{
				name:       "root serves index",
				method:     http.MethodGet,
				target:     "/",
				wantStatus: http.StatusOK,
				wantBody:   indexContent,
			},
			{
				name:       "existing asset serves file",
				method:     http.MethodGet,
				target:     "/" + assetPath,
				wantStatus: http.StatusOK,
				wantBody:   assetContent,
			},
			{
				name:       "spa route falls back to index",
				method:     http.MethodGet,
				target:     "/finance/overview",
				wantStatus: http.StatusOK,
				wantBody:   indexContent,
			},
			{
				name:       "single segment spa route falls back to index",
				method:     http.MethodGet,
				target:     "/finance",
				wantStatus: http.StatusOK,
				wantBody:   indexContent,
			},
			{
				name:       "nested directory index is served",
				method:     http.MethodGet,
				target:     "/nested/",
				wantStatus: http.StatusOK,
				wantBody:   nestedIndexContent,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				response := makeRequest(t, handler, testCase.method, testCase.target)
				require.Equal(t, testCase.wantStatus, response.Code)
				require.Equal(t, testCase.wantBody, response.Body.String())
			})
		}
	})

	t.Run("rejects non-spa fallbacks", func(t *testing.T) {
		indexContent := fake.Lorem().Sentence(5)
		assetDirectory := "assets"
		handler := newUIHandler(fstest.MapFS{
			"index.html":                         {Data: []byte(indexContent)},
			assetDirectory + "/app.js":           {Data: []byte(fake.Lorem().Sentence(2))},
			"enable-banking/callback/index.html": {Data: []byte(fake.Lorem().Sentence(2))},
		})

		for _, testCase := range []struct {
			name       string
			method     string
			target     string
			wantStatus int
		}{
			{
				name:       "existing directory without nested index returns not found",
				method:     http.MethodGet,
				target:     "/assets/",
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "missing asset-like path returns not found",
				method:     http.MethodGet,
				target:     "/assets/missing.js",
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "missing asset namespace path returns not found",
				method:     http.MethodGet,
				target:     "/assets/missing",
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "missing api path returns not found",
				method:     http.MethodGet,
				target:     "/api/v1/unknown",
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "missing callback namespace path returns not found",
				method:     http.MethodGet,
				target:     "/enable-banking/missing",
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "non get request returns not found",
				method:     http.MethodPost,
				target:     "/finance/overview",
				wantStatus: http.StatusNotFound,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				response := makeRequest(t, handler, testCase.method, testCase.target)
				require.Equal(t, testCase.wantStatus, response.Code)
			})
		}
	})

	t.Run("returns not found when fallback index is unavailable", func(t *testing.T) {
		handler := newUIHandler(fstest.MapFS{
			"assets/app.js": {Data: []byte(fake.Lorem().Sentence(2))},
		})

		response := makeRequest(t, handler, http.MethodGet, "/finance/overview")

		require.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("returns not found when asked to serve a directory as a file", func(t *testing.T) {
		handler := &uiHandler{files: fstest.MapFS{
			"assets/app.js": {Data: []byte(fake.Lorem().Sentence(2))},
		}}
		request := httptest.NewRequest(http.MethodGet, "/assets", http.NoBody)
		response := httptest.NewRecorder()

		handler.serveFile(response, request, "assets")

		require.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestResolveEmbeddedUIFiles(t *testing.T) {
	fake := faker.New()

	t.Run("returns nil when embedded ui dist directory is missing", func(t *testing.T) {
		uiFiles := resolveEmbeddedUIFiles(fstest.MapFS{})

		require.Nil(t, uiFiles)
	})

	t.Run("returns nil when embedded ui sub filesystem cannot be created", func(t *testing.T) {
		uiFiles := resolveEmbeddedUIFiles(subErrorFS{})

		require.Nil(t, uiFiles)
	})

	t.Run("returns nil when generated dist index is missing", func(t *testing.T) {
		uiFiles := resolveEmbeddedUIFiles(fstest.MapFS{
			"embeddedui/placeholder/index.html": {Data: []byte(fake.Lorem().Sentence(3))},
		})

		require.Nil(t, uiFiles)
	})

	t.Run("returns nil when generated dist is missing index.html", func(t *testing.T) {
		uiFiles := resolveEmbeddedUIFiles(fstest.MapFS{
			"embeddedui/placeholder/index.html": {Data: []byte(fake.Lorem().Sentence(3))},
			"embeddedui/dist/assets/app.js":     {Data: []byte(fake.Lorem().Sentence(2))},
		})

		require.Nil(t, uiFiles)
	})

	t.Run("returns generated dist when index is present", func(t *testing.T) {
		wantIndexContent := fake.Lorem().Sentence(4)
		wantAssetContent := fake.Lorem().Sentence(2)
		uiFiles := resolveEmbeddedUIFiles(fstest.MapFS{
			"embeddedui/placeholder/index.html": {Data: []byte(fake.Lorem().Sentence(3))},
			"embeddedui/dist/index.html":        {Data: []byte(wantIndexContent)},
			"embeddedui/dist/assets/app.js":     {Data: []byte(wantAssetContent)},
		})

		require.NotNil(t, uiFiles)

		indexContent, err := fs.ReadFile(uiFiles, "index.html")
		require.NoError(t, err)
		require.Equal(t, wantIndexContent, string(indexContent))

		assetContent, err := fs.ReadFile(uiFiles, "assets/app.js")
		require.NoError(t, err)
		require.Equal(t, wantAssetContent, string(assetContent))
	})
}

func TestMountUIRoutes(t *testing.T) {
	fake := faker.New()

	makeResponse := func(t *testing.T, embeddedUIFiles fs.FS) *httptest.ResponseRecorder {
		t.Helper()
		router := server.NewHTTPRouter(server.HTTPRouterDeps{
			Middleware: func(h http.Handler) http.Handler { return h },
		})
		mountUIRoutesWithEmbedded(telemetry.RootTestLogger(), router, embeddedUIFiles)
		request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	t.Run("uses embedded ui when generated dist is available", func(t *testing.T) {
		embeddedIndexContent := fake.Lorem().Sentence(5)
		response := makeResponse(t, fstest.MapFS{
			"index.html": {Data: []byte(embeddedIndexContent)},
		})

		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, embeddedIndexContent, response.Body.String())
	})

	t.Run("keeps api-only mode when no ui source exists", func(t *testing.T) {
		response := makeResponse(t, nil)

		require.Equal(t, http.StatusNotFound, response.Code)
	})
}
