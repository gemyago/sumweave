package http

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/gemyago/signal-foundry/apps/signal-foundry/internal/api/http/server"
)

type uiHandler struct {
	files fs.FS
}

func newUIHandler(uiFiles fs.FS) http.Handler {
	return &uiHandler{
		files: uiFiles,
	}
}

func (handler *uiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	requestPath := normalizeUIRequestPath(r.URL.Path)
	if requestPath == "" {
		handler.serveFile(w, r, "index.html")
		return
	}

	if resolvedPath, ok := handler.resolveExistingPath(requestPath); ok {
		handler.serveFile(w, r, resolvedPath)
		return
	}

	if !handler.shouldServeSPAIndex(requestPath) {
		http.NotFound(w, r)
		return
	}

	handler.serveFile(w, r, "index.html")
}

func (handler *uiHandler) resolveExistingPath(requestPath string) (string, bool) {
	fileInfo, err := fs.Stat(handler.files, requestPath)
	if err != nil {
		return "", false
	}
	if !fileInfo.IsDir() {
		return requestPath, true
	}

	nestedIndexPath := path.Join(requestPath, "index.html")
	indexInfo, err := fs.Stat(handler.files, nestedIndexPath)
	if err != nil || indexInfo.IsDir() {
		return "", false
	}

	return nestedIndexPath, true
}

func (handler *uiHandler) shouldServeSPAIndex(requestPath string) bool {
	if isReservedBackendPath(requestPath) {
		return false
	}
	if isAssetLikePath(requestPath) {
		return false
	}

	firstSegment := pathFirstSegment(requestPath)
	if firstSegment == requestPath {
		segmentInfo, err := fs.Stat(handler.files, firstSegment)
		if err != nil {
			return errors.Is(err, fs.ErrNotExist)
		}

		return !segmentInfo.IsDir()
	}

	segmentInfo, err := fs.Stat(handler.files, firstSegment)
	if err != nil {
		return errors.Is(err, fs.ErrNotExist)
	}

	return !segmentInfo.IsDir()
}

func (handler *uiHandler) serveFile(w http.ResponseWriter, r *http.Request, filePath string) {
	uiFile, err := handler.files.Open(strings.TrimPrefix(filePath, "/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = uiFile.Close() }()

	fileInfo, err := uiFile.Stat()
	if err != nil || fileInfo.IsDir() {
		http.NotFound(w, r)
		return
	}

	if readSeeker, ok := uiFile.(io.ReadSeeker); ok {
		http.ServeContent(w, r, path.Base(filePath), fileInfo.ModTime(), readSeeker)
		return
	}

	http.NotFound(w, r)
}

func mountUIRoutes(logger *slog.Logger, router *server.HTTPRouter) {
	mountUIRoutesWithEmbedded(logger, router, getEmbeddedUIFiles())
}

func mountUIRoutesWithEmbedded(
	logger *slog.Logger,
	router *server.HTTPRouter,
	embeddedUIFiles fs.FS,
) {
	uiFiles, ok := resolveUIFiles(logger, embeddedUIFiles)
	if !ok {
		logger.Info("embedded UI assets unavailable; HTTP server remains API-only")
		return
	}

	router.Handle("/", newUIHandler(uiFiles))
}

func resolveUIFiles(_ *slog.Logger, embeddedUIFiles fs.FS) (fs.FS, bool) {
	if embeddedUIFiles != nil {
		return embeddedUIFiles, true
	}

	return nil, false
}

func normalizeUIRequestPath(requestPath string) string {
	cleanPath := path.Clean("/" + requestPath)
	if cleanPath == "/" {
		return ""
	}

	return strings.TrimPrefix(cleanPath, "/")
}

func isAssetLikePath(requestPath string) bool {
	for _, segment := range strings.Split(requestPath, "/") {
		if strings.Contains(segment, ".") {
			return true
		}
	}

	return false
}

func isReservedBackendPath(requestPath string) bool {
	switch pathFirstSegment(requestPath) {
	case "api", "enable-banking":
		return true
	default:
		return false
	}
}

func pathFirstSegment(requestPath string) string {
	segment, _, _ := strings.Cut(requestPath, "/")
	return segment
}
