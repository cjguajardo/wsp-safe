package archive

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cgc/wsp-safe/internal/filter"
)

func TestEncryptedArchiveRoundTrip(t *testing.T) {
	directory := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	store, err := NewEncrypted(directory, key)
	if err != nil {
		t.Fatalf("NewEncrypted() error = %v", err)
	}
	message := filter.Message{
		ID:        "mensaje/privado",
		ChatID:    "persona@s.whatsapp.net",
		SenderID:  "56911111111@s.whatsapp.net",
		Kind:      filter.KindImage,
		MIMEType:  "image/jpeg",
		Text:      "contenido confidencial",
		Media:     []byte("bytes-confidenciales"),
		Timestamp: time.Unix(1_750_000_000, 0),
	}

	decision := filter.Decision{Delete: true, Reason: filter.ReasonSexual, Result: filter.Result{SexualScore: 0.9}}
	if err := store.Archive(context.Background(), message, decision); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.wsp-safe"))
	if err != nil || len(files) != 1 {
		t.Fatalf("archivos = %v, error = %v", files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, secret := range [][]byte{[]byte(message.Text), message.Media, []byte(message.SenderID)} {
		if bytes.Contains(raw, secret) {
			t.Fatalf("el archivo cifrado expone %q", secret)
		}
	}
	info, err := os.Stat(files[0])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos = %v, error = %v", info.Mode().Perm(), err)
	}
	recovered, err := DecryptFile(files[0], key)
	if err != nil {
		t.Fatalf("DecryptFile() error = %v", err)
	}
	if recovered.Message.Text != message.Text || !bytes.Equal(recovered.Message.Media, message.Media) || recovered.Message.SenderID != message.SenderID || recovered.Decision.Reason != filter.ReasonSexual {
		t.Errorf("mensaje recuperado = %+v", recovered)
	}
}

func TestEncryptedArchiveRejectsInvalidKey(t *testing.T) {
	if _, err := NewEncrypted(t.TempDir(), []byte("corta")); err == nil {
		t.Fatal("NewEncrypted() error = nil, se esperaba error")
	}
}
