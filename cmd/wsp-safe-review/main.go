package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	securearchive "github.com/cgc/wsp-safe/internal/archive"
	"github.com/cgc/wsp-safe/internal/filter"
)

func main() {
	if err := runReview(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runReview(arguments []string, getenv func(string) string) error {
	flags := flag.NewFlagSet("wsp-safe-review", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	archiveFile := flags.String("file", "", "archivo cifrado que se revisará")
	outputDir := flags.String("output", "", "directorio privado donde se exportará")
	if err := flags.Parse(arguments); err != nil {
		return fmt.Errorf("argumentos inválidos: %w", err)
	}
	if strings.TrimSpace(*archiveFile) == "" || strings.TrimSpace(*outputDir) == "" {
		return errors.New("-file y -output son obligatorios")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(getenv("WSP_ARCHIVE_KEY")))
	if err != nil || len(key) != 32 {
		return errors.New("WSP_ARCHIVE_KEY debe contener una clave de 32 bytes codificada en base64")
	}
	record, err := securearchive.DecryptFile(*archiveFile, key)
	if err != nil {
		return fmt.Errorf("descifrar archivo: %w", err)
	}
	return exportReview(*outputDir, record)
}

func exportReview(directory string, record securearchive.Record) error {
	message := record.Message
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("crear directorio de revisión: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("restringir directorio de revisión: %w", err)
	}
	metadata := struct {
		ID         string          `json:"id"`
		ChatID     string          `json:"chat_id"`
		SenderID   string          `json:"sender_id"`
		Kind       filter.Kind     `json:"kind"`
		MIMEType   string          `json:"mime_type"`
		Text       string          `json:"text"`
		Timestamp  time.Time       `json:"timestamp"`
		HasContent bool            `json:"has_content"`
		Decision   filter.Decision `json:"decision"`
	}{
		ID: message.ID, ChatID: message.ChatID, SenderID: message.SenderID,
		Kind: message.Kind, MIMEType: message.MIMEType, Text: message.Text,
		Timestamp: message.Timestamp, HasContent: len(message.Media) > 0,
		Decision: record.Decision,
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("codificar revisión: %w", err)
	}
	if err := writePrivateFile(filepath.Join(directory, "mensaje.json"), encoded); err != nil {
		return err
	}
	if len(message.Media) > 0 {
		if err := writePrivateFile(filepath.Join(directory, "contenido"+mediaExtension(message.MIMEType)), message.Media); err != nil {
			return err
		}
	}
	return nil
}

func writePrivateFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("crear archivo de revisión: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("escribir archivo de revisión: %w", err)
	}
	return nil
}

func mediaExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	default:
		return ".bin"
	}
}
