package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Config struct {
	ClassifierURL   string
	ClassifierToken string
	SessionDB       string
	SexualThreshold float64
	DeleteUncertain bool
	DeleteOnError   bool
	LogDecisions    bool
	ArchiveDeleted  bool
	ArchiveKey      []byte
	ArchiveDir      string
	MaxMediaBytes   uint64
	Workers         int
}

func Load(getenv func(string) string) (Config, error) {
	config := Config{
		ClassifierURL:   strings.TrimSpace(getenv("WSP_CLASSIFIER_URL")),
		ClassifierToken: getenv("WSP_CLASSIFIER_TOKEN"),
		SessionDB:       SessionDB(getenv),
		SexualThreshold: 0.25,
		DeleteUncertain: true,
		DeleteOnError:   true,
		ArchiveDir:      valueOr(getenv("WSP_ARCHIVE_DIR"), "/data/deleted"),
		MaxMediaBytes:   20 << 20,
		Workers:         1,
	}
	if config.ClassifierURL == "" {
		return Config{}, errors.New("WSP_CLASSIFIER_URL is required")
	}

	var err error
	if raw := strings.TrimSpace(getenv("WSP_SEXUAL_THRESHOLD")); raw != "" {
		config.SexualThreshold, err = strconv.ParseFloat(raw, 64)
		if err != nil || config.SexualThreshold <= 0 || config.SexualThreshold > 1 {
			return Config{}, fmt.Errorf("WSP_SEXUAL_THRESHOLD must be greater than zero and at most one")
		}
	}
	if raw := strings.TrimSpace(getenv("WSP_DELETE_UNCERTAIN")); raw != "" {
		config.DeleteUncertain, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WSP_DELETE_UNCERTAIN: %w", err)
		}
	}
	if raw := strings.TrimSpace(getenv("WSP_DELETE_ON_ERROR")); raw != "" {
		config.DeleteOnError, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WSP_DELETE_ON_ERROR: %w", err)
		}
	}
	if raw := strings.TrimSpace(getenv("WSP_LOG_DECISIONS")); raw != "" {
		config.LogDecisions, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WSP_LOG_DECISIONS: %w", err)
		}
	}
	if raw := strings.TrimSpace(getenv("WSP_ARCHIVE_DELETED")); raw != "" {
		config.ArchiveDeleted, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WSP_ARCHIVE_DELETED: %w", err)
		}
	}
	if config.ArchiveDeleted {
		rawKey := strings.TrimSpace(getenv("WSP_ARCHIVE_KEY"))
		config.ArchiveKey, err = base64.StdEncoding.DecodeString(rawKey)
		if err != nil || len(config.ArchiveKey) != 32 {
			return Config{}, errors.New("WSP_ARCHIVE_KEY must be a base64-encoded 32-byte key when archiving is enabled")
		}
	}
	if raw := strings.TrimSpace(getenv("WSP_MAX_MEDIA_BYTES")); raw != "" {
		config.MaxMediaBytes, err = strconv.ParseUint(raw, 10, 64)
		if err != nil || config.MaxMediaBytes == 0 {
			return Config{}, errors.New("WSP_MAX_MEDIA_BYTES must be a positive integer")
		}
	}
	if raw := strings.TrimSpace(getenv("WSP_WORKERS")); raw != "" {
		config.Workers, err = strconv.Atoi(raw)
		if err != nil || config.Workers < 1 || config.Workers > 8 {
			return Config{}, errors.New("WSP_WORKERS must be an integer between 1 and 8")
		}
	}
	return config, nil
}

func SessionDB(getenv func(string) string) string {
	return valueOr(getenv("WSP_SESSION_DB"), "file:wsp-safe.db?_foreign_keys=on")
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
