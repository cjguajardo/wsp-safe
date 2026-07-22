// El paquete archive almacena contenido eliminado mediante cifrado autenticado.
package archive

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cgc/wsp-safe/internal/filter"
)

const archiveHeader = "WSPARCH1"

type Encrypted struct {
	directory string
	aead      cipher.AEAD
}

type Record struct {
	Message  filter.Message  `json:"message"`
	Decision filter.Decision `json:"decision"`
}

func NewEncrypted(directory string, key []byte) (*Encrypted, error) {
	if len(key) != 32 {
		return nil, errors.New("archive key must contain exactly 32 bytes")
	}
	if directory == "" {
		return nil, errors.New("archive directory is required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create archive cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create archive GCM: %w", err)
	}
	return &Encrypted{directory: directory, aead: aead}, nil
}

func (e *Encrypted) Archive(ctx context.Context, message filter.Message, decision filter.Decision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plaintext, err := json.Marshal(Record{Message: message, Decision: decision})
	if err != nil {
		return fmt.Errorf("encode archived message: %w", err)
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("create archive nonce: %w", err)
	}
	header := []byte(archiveHeader)
	ciphertext := e.aead.Seal(nil, nonce, plaintext, header)
	contents := append(append(header, nonce...), ciphertext...)

	if err := os.MkdirAll(e.directory, 0o700); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}
	if err := os.Chmod(e.directory, 0o700); err != nil {
		return fmt.Errorf("restrict archive directory: %w", err)
	}
	path := filepath.Join(e.directory, archiveFilename(message))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}
	completed := false
	defer func() {
		_ = file.Close()
		if !completed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write archive file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync archive file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close archive file: %w", err)
	}
	completed = true
	return nil
}

func DecryptFile(path string, key []byte) (Record, error) {
	store, err := NewEncrypted(filepath.Dir(path), key)
	if err != nil {
		return Record{}, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("read archive file: %w", err)
	}
	header := []byte(archiveHeader)
	minimum := len(header) + store.aead.NonceSize() + store.aead.Overhead()
	if len(contents) < minimum || string(contents[:len(header)]) != archiveHeader {
		return Record{}, errors.New("invalid archive file")
	}
	nonceEnd := len(header) + store.aead.NonceSize()
	plaintext, err := store.aead.Open(nil, contents[len(header):nonceEnd], contents[nonceEnd:], header)
	if err != nil {
		return Record{}, errors.New("archive authentication failed")
	}
	var record Record
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return Record{}, fmt.Errorf("decode archived message: %w", err)
	}
	return record, nil
}

func archiveFilename(message filter.Message) string {
	digest := sha256.Sum256([]byte(message.ChatID + "\x00" + message.ID))
	timestamp := message.Timestamp.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return timestamp.Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(digest[:8]) + ".wsp-safe"
}
