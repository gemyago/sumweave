//go:build postgres_test

package jobs

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func createWithDB(ctx context.Context, db *gorm.DB, tableName string, job Job) error {
	model := newJobModel(job)
	if err := db.WithContext(ctx).Table(tableName).Create(&model).Error; err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}
