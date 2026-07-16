package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StoreRawPayloadBody stores immutable raw payload bytes and returns a stable reference.
func (s *DatabaseStore) StoreRawPayloadBody(
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

	checksum := sha256.Sum256(body)
	hash := hex.EncodeToString(checksum[:])
	ref := makeRawPayloadBodyRef(canonicalPayloadID)
	model := rawPayloadBodyModel{
		Ref:       ref,
		PayloadID: canonicalPayloadID,
		BodyHash:  hash,
		Body:      append([]byte(nil), body...),
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model).Error; err != nil {
		return RawPayloadBody{}, fmt.Errorf("store raw payload body: %w", err)
	}

	var persisted rawPayloadBodyModel
	if err := s.db.WithContext(ctx).Where("payload_id = ?", canonicalPayloadID).First(&persisted).Error; err != nil {
		return RawPayloadBody{}, fmt.Errorf("read stored raw payload body: %w", err)
	}
	return RawPayloadBody{Ref: persisted.Ref, Hash: persisted.BodyHash}, nil
}

// ReadRawPayloadBody loads raw payload bytes by blob reference.
func (s *DatabaseStore) ReadRawPayloadBody(ctx context.Context, ref string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	canonicalRef := strings.TrimSpace(ref)
	if canonicalRef == "" {
		return nil, validationError("raw payload body ref is required")
	}

	var row rawPayloadBodyModel
	if err := s.db.WithContext(ctx).Select("body").Where("ref = ?", canonicalRef).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRawPayloadNotFound
		}
		return nil, fmt.Errorf("read raw payload body: %w", err)
	}
	return append([]byte(nil), row.Body...), nil
}

func (s *DatabaseStore) readRawPayloadBodyPreview(
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

	canonicalRef := strings.TrimSpace(ref)
	if canonicalRef == "" {
		return rawPayloadBodyPreview{}, validationError("raw payload body ref is required")
	}

	type previewRow struct {
		SizeBytes int    `gorm:"column:size_bytes"`
		Preview   []byte `gorm:"column:preview"`
	}
	var row previewRow
	query := s.db.WithContext(ctx).Table((rawPayloadBodyModel{}).TableName(s.db.NamingStrategy))
	if s.db.Dialector.Name() == "postgres" {
		query = query.Select(
			"octet_length(body) AS size_bytes, substring(body FROM 1 FOR ?) AS preview",
			limit+1,
		)
	} else {
		query = query.Select("length(body) AS size_bytes, substr(body, 1, ?) AS preview", limit+1)
	}
	result := query.Where("ref = ?", canonicalRef).Scan(&row)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return rawPayloadBodyPreview{}, ErrRawPayloadNotFound
		}
		return rawPayloadBodyPreview{}, fmt.Errorf("read raw payload body preview: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return rawPayloadBodyPreview{}, ErrRawPayloadNotFound
	}
	previewEnd := min(len(row.Preview), limit)
	return rawPayloadBodyPreview{
		sizeBytes: row.SizeBytes,
		preview:   append([]byte(nil), row.Preview[:previewEnd]...),
		truncated: row.SizeBytes > limit,
	}, nil
}

func makeRawPayloadBodyRef(payloadID string) string {
	sum := sha256.Sum256([]byte(payloadID))
	return hex.EncodeToString(sum[:])
}
