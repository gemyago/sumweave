package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalRawPayloadBlobStore stores raw payload bodies on the local filesystem.
type LocalRawPayloadBlobStore struct {
	basePath string
}

// NewLocalRawPayloadBlobStore constructs a local raw payload blob store rooted at basePath.
func NewLocalRawPayloadBlobStore(basePath string) (*LocalRawPayloadBlobStore, error) {
	canonicalBasePath := strings.TrimSpace(basePath)
	if canonicalBasePath == "" {
		return nil, errors.New("base path is required")
	}

	return &LocalRawPayloadBlobStore{basePath: canonicalBasePath}, nil
}

// StoreRawPayloadBody stores immutable raw payload bytes and returns a stable reference.
func (s *LocalRawPayloadBlobStore) StoreRawPayloadBody(
	ctx context.Context,
	payloadID string,
	body []byte,
) (RawPayloadBody, error) {
	if err := ctx.Err(); err != nil {
		return RawPayloadBody{}, err
	}

	canonicalPayloadID := strings.TrimSpace(payloadID)
	if canonicalPayloadID == "" {
		return RawPayloadBody{}, validationError("raw payload id is required")
	}
	if len(body) == 0 {
		return RawPayloadBody{}, validationError("raw payload body is required")
	}

	ref := makeRawPayloadBlobRef(canonicalPayloadID)
	fullPath, err := s.blobPath(ref)
	if err != nil {
		return RawPayloadBody{}, err
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(fullPath), 0o700); mkdirErr != nil {
		return RawPayloadBody{}, fmt.Errorf("create raw payload blob directory: %w", mkdirErr)
	}

	checksum := sha256.Sum256(body)
	hash := hex.EncodeToString(checksum[:])

	if storeErr := writeRawPayloadBlobAtomic(fullPath, body); storeErr == nil {
		return RawPayloadBody{Ref: ref, Hash: hash}, nil
	} else if !errors.Is(storeErr, os.ErrExist) {
		return RawPayloadBody{}, storeErr
	}

	existingBody, readErr := os.ReadFile(fullPath)
	if readErr != nil {
		return RawPayloadBody{}, fmt.Errorf("read existing raw payload blob: %w", readErr)
	}
	existingChecksum := sha256.Sum256(existingBody)
	return RawPayloadBody{Ref: ref, Hash: hex.EncodeToString(existingChecksum[:])}, nil
}

func writeRawPayloadBlobAtomic(fullPath string, body []byte) error {
	tempFile, err := os.CreateTemp(filepath.Dir(fullPath), filepath.Base(fullPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp raw payload blob: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if written, writeErr := tempFile.Write(body); writeErr != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp raw payload blob: %w", writeErr)
	} else if written != len(body) {
		_ = tempFile.Close()
		return fmt.Errorf("write temp raw payload blob: %w", io.ErrShortWrite)
	}
	if syncErr := tempFile.Sync(); syncErr != nil {
		_ = tempFile.Close()
		return fmt.Errorf("sync temp raw payload blob: %w", syncErr)
	}
	if closeErr := tempFile.Close(); closeErr != nil {
		return fmt.Errorf("close temp raw payload blob: %w", closeErr)
	}
	if linkErr := os.Link(tempPath, fullPath); linkErr != nil {
		return fmt.Errorf("publish raw payload blob: %w", linkErr)
	}

	return nil
}

// ReadRawPayloadBody loads raw payload bytes by blob reference.
func (s *LocalRawPayloadBlobStore) ReadRawPayloadBody(ctx context.Context, ref string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fullPath, err := s.blobPath(ref)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read raw payload blob: %w", err)
	}
	return body, nil
}

func (s *LocalRawPayloadBlobStore) readRawPayloadBodyPreview(
	ctx context.Context,
	ref string,
	limit int,
) (rawPayloadBodyPreview, error) {
	if err := ctx.Err(); err != nil {
		return rawPayloadBodyPreview{}, err
	}
	if limit < 0 {
		return rawPayloadBodyPreview{}, validationError("raw payload preview limit must be zero or greater")
	}

	fullPath, err := s.blobPath(ref)
	if err != nil {
		return rawPayloadBodyPreview{}, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return rawPayloadBodyPreview{}, fmt.Errorf("open raw payload blob: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return rawPayloadBodyPreview{}, fmt.Errorf("stat raw payload blob: %w", err)
	}

	previewLimit := int64(limit)
	if limit < int(^uint(0)>>1) {
		previewLimit++
	}

	preview, err := io.ReadAll(io.LimitReader(file, previewLimit))
	if err != nil {
		return rawPayloadBodyPreview{}, fmt.Errorf("read raw payload blob preview: %w", err)
	}

	truncated := len(preview) > limit
	if truncated {
		preview = preview[:limit]
	}

	sizeBytes := max(int(info.Size()), len(preview))
	if truncated && sizeBytes < limit+1 {
		sizeBytes = limit + 1
	}

	return rawPayloadBodyPreview{
		sizeBytes: sizeBytes,
		preview:   preview,
		truncated: truncated,
	}, nil
}

func makeRawPayloadBlobRef(payloadID string) string {
	sum := sha256.Sum256([]byte(payloadID))
	key := hex.EncodeToString(sum[:])
	return filepath.ToSlash(filepath.Join(key[:2], key+".blob"))
}

func (s *LocalRawPayloadBlobStore) blobPath(ref string) (string, error) {
	canonicalRef := strings.TrimSpace(ref)
	if canonicalRef == "" {
		return "", validationError("raw payload body ref is required")
	}
	if filepath.IsAbs(canonicalRef) {
		return "", validationError("raw payload body ref must be relative")
	}

	fullPath := filepath.Join(s.basePath, filepath.FromSlash(canonicalRef))
	cleanBase := filepath.Clean(s.basePath)
	cleanPath := filepath.Clean(fullPath)
	if cleanPath != cleanBase && !strings.HasPrefix(cleanPath, cleanBase+string(os.PathSeparator)) {
		return "", validationError("raw payload body ref must stay within blob store base path")
	}

	return cleanPath, nil
}
