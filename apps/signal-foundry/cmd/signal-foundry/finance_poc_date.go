package main

import (
	"fmt"
	"strings"
	"time"
)

const financePOCDateLayout = "2006-01-02"

func parseFinancePOCDate(flagName string, value string) (time.Time, error) {
	parsed, err := time.Parse(financePOCDateLayout, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", flagName, err)
	}
	return parsed, nil
}

func parseFinancePOCInclusiveEndDate(flagName string, value string) (time.Time, error) {
	parsed, err := parseFinancePOCDate(flagName, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.AddDate(0, 0, 1).Add(-time.Second), nil
}
