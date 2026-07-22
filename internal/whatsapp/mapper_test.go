package whatsapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/cgc/wsp-safe/internal/filter"
)

func TestMapperMapsAndDownloadsImage(t *testing.T) {
	downloader := &fakeDownloader{data: []byte("image")}
	mapper := NewMapper(downloader, 1024)
	message := eventMessage(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		Mimetype:   proto.String("image/jpeg"),
		FileLength: proto.Uint64(5),
	}})

	got, err := mapper.Map(context.Background(), message)
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	if got.Kind != filter.KindImage {
		t.Errorf("kind = %q, want image", got.Kind)
	}
	if string(got.Media) != "image" {
		t.Errorf("media = %q, want image", got.Media)
	}
	if downloader.calls != 1 {
		t.Errorf("download calls = %d, want 1", downloader.calls)
	}
}

func TestMapperMarksOversizedOrFailedMediaUnavailable(t *testing.T) {
	tests := []struct {
		name        string
		fileLength  uint64
		downloadErr error
	}{
		{name: "oversized", fileLength: 2048},
		{name: "download failure", fileLength: 10, downloadErr: errors.New("download failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			downloader := &fakeDownloader{err: tt.downloadErr}
			mapper := NewMapper(downloader, 1024)
			message := eventMessage(&waE2E.Message{VideoMessage: &waE2E.VideoMessage{
				Mimetype:   proto.String("video/mp4"),
				FileLength: proto.Uint64(tt.fileLength),
			}})

			got, err := mapper.Map(context.Background(), message)
			if err != nil {
				t.Fatalf("Map() error = %v", err)
			}
			if !got.Unavailable {
				t.Error("unavailable = false, want true")
			}
		})
	}
}

func TestMapperMapsTextWithoutDownload(t *testing.T) {
	downloader := &fakeDownloader{}
	mapper := NewMapper(downloader, 1024)
	got, err := mapper.Map(context.Background(), eventMessage(&waE2E.Message{
		Conversation: proto.String("hello"),
	}))
	if err != nil {
		t.Fatalf("Map() error = %v", err)
	}
	if got.Kind != filter.KindText || got.Text != "hello" {
		t.Errorf("message = %#v, want text hello", got)
	}
	if downloader.calls != 0 {
		t.Errorf("download calls = %d, want 0", downloader.calls)
	}
}

func eventMessage(message *waE2E.Message) *events.Message {
	chat, _ := types.ParseJID("120363000000000000@g.us")
	sender, _ := types.ParseJID("56911111111@s.whatsapp.net")
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsGroup: true},
			ID:            "message-id",
			Timestamp:     time.Unix(1_750_000_000, 0),
		},
		Message: message,
	}
}

type fakeDownloader struct {
	data  []byte
	err   error
	calls int
}

func (f *fakeDownloader) DownloadAny(context.Context, *waE2E.Message) ([]byte, error) {
	f.calls++
	return f.data, f.err
}
