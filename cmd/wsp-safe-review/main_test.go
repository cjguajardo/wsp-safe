package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	securearchive "github.com/cgc/wsp-safe/internal/archive"
	"github.com/cgc/wsp-safe/internal/filter"
)

func TestRunReviewExportsPlaintextOnlyOnExplicitRequest(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	archiveDir := t.TempDir()
	store, err := securearchive.NewEncrypted(archiveDir, key)
	if err != nil {
		t.Fatalf("NewEncrypted() error = %v", err)
	}
	if err := store.Archive(context.Background(), filter.Message{
		ID: "m1", SenderID: "56911111111@s.whatsapp.net", Kind: filter.KindImage,
		MIMEType: "image/jpeg", Text: "texto para revisión", Media: []byte("imagen"),
		Timestamp: time.Now(),
	}, filter.Decision{Delete: true, Reason: filter.ReasonSexual}); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	archives, _ := filepath.Glob(filepath.Join(archiveDir, "*.wsp-safe"))
	outputDir := filepath.Join(t.TempDir(), "revision")
	getenv := func(keyName string) string {
		if keyName == "WSP_ARCHIVE_KEY" {
			return base64.StdEncoding.EncodeToString(key)
		}
		return ""
	}

	if err := runReview([]string{"-file", archives[0], "-output", outputDir}, getenv); err != nil {
		t.Fatalf("runReview() error = %v", err)
	}
	metadata, err := os.ReadFile(filepath.Join(outputDir, "mensaje.json"))
	if err != nil || !bytes.Contains(metadata, []byte("texto para revisión")) {
		t.Fatalf("metadatos = %q, error = %v", metadata, err)
	}
	media, err := os.ReadFile(filepath.Join(outputDir, "contenido.jpg"))
	if err != nil || string(media) != "imagen" {
		t.Fatalf("contenido = %q, error = %v", media, err)
	}
}
