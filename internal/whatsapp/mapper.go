package whatsapp

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/cgc/wsp-safe/internal/filter"
)

type MediaDownloader interface {
	DownloadAny(context.Context, *waE2E.Message) ([]byte, error)
}

type Mapper struct {
	downloader MediaDownloader
	maxBytes   uint64
}

func NewMapper(downloader MediaDownloader, maxBytes uint64) *Mapper {
	return &Mapper{downloader: downloader, maxBytes: maxBytes}
}

func (m *Mapper) Map(ctx context.Context, event *events.Message) (filter.Message, error) {
	if event == nil || event.Message == nil {
		return filter.Message{}, errors.New("WhatsApp message is nil")
	}
	message := filter.Message{
		ID:        string(event.Info.ID),
		ChatID:    event.Info.Chat.String(),
		SenderID:  event.Info.Sender.String(),
		FromMe:    event.Info.IsFromMe,
		Timestamp: event.Info.Timestamp,
	}

	var fileLength uint64
	switch {
	case event.Message.GetImageMessage() != nil:
		media := event.Message.GetImageMessage()
		message.Kind = filter.KindImage
		message.MIMEType = media.GetMimetype()
		message.Text = media.GetCaption()
		fileLength = media.GetFileLength()
	case event.Message.GetVideoMessage() != nil:
		media := event.Message.GetVideoMessage()
		message.Kind = filter.KindVideo
		message.MIMEType = media.GetMimetype()
		message.Text = media.GetCaption()
		fileLength = media.GetFileLength()
	case event.Message.GetStickerMessage() != nil:
		media := event.Message.GetStickerMessage()
		message.Kind = filter.KindSticker
		message.MIMEType = media.GetMimetype()
		fileLength = media.GetFileLength()
	case event.Message.GetConversation() != "":
		message.Kind = filter.KindText
		message.Text = event.Message.GetConversation()
		return message, nil
	case event.Message.GetExtendedTextMessage() != nil:
		message.Kind = filter.KindText
		message.Text = event.Message.GetExtendedTextMessage().GetText()
		return message, nil
	default:
		return message, nil
	}

	if m.maxBytes > 0 && fileLength > m.maxBytes {
		message.Unavailable = true
		return message, nil
	}
	if m.downloader == nil {
		message.Unavailable = true
		return message, nil
	}
	media, err := m.downloader.DownloadAny(ctx, event.Message)
	if err != nil || (m.maxBytes > 0 && uint64(len(media)) > m.maxBytes) {
		message.Unavailable = true
		return message, nil
	}
	message.Media = media
	return message, nil
}
