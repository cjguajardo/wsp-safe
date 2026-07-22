package whatsapp

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"google.golang.org/protobuf/proto"

	"github.com/cgc/wsp-safe/internal/filter"
)

type AppStateSender interface {
	SendAppState(context.Context, appstate.PatchInfo) error
}

type Deleter struct {
	sender AppStateSender
}

func NewDeleter(sender AppStateSender) *Deleter {
	return &Deleter{sender: sender}
}

func (d *Deleter) DeleteForMe(ctx context.Context, message filter.Message) error {
	patch, err := BuildDeleteForMePatch(message)
	if err != nil {
		return err
	}
	return d.sender.SendAppState(ctx, patch)
}

// BuildDeleteForMePatch creates the regular_high app-state mutation used by
// WhatsApp linked devices for the local "Delete for me" operation.
func BuildDeleteForMePatch(message filter.Message) (appstate.PatchInfo, error) {
	if message.ID == "" {
		return appstate.PatchInfo{}, errors.New("message ID is required")
	}
	if message.ChatID == "" {
		return appstate.PatchInfo{}, errors.New("chat ID is required")
	}
	if message.SenderID == "" {
		return appstate.PatchInfo{}, errors.New("sender ID is required")
	}
	if message.Timestamp.IsZero() {
		return appstate.PatchInfo{}, errors.New("message timestamp is required")
	}

	fromMe := "0"
	if message.FromMe {
		fromMe = "1"
	}
	senderID := message.SenderID
	if message.ChatID == message.SenderID {
		senderID = "0"
	}

	return appstate.PatchInfo{
		Type: appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{{
			Index: []string{
				appstate.IndexDeleteMessageForMe,
				message.ChatID,
				message.ID,
				fromMe,
				senderID,
			},
			Version: 2,
			Value: &waSyncAction.SyncActionValue{
				DeleteMessageForMeAction: &waSyncAction.DeleteMessageForMeAction{
					DeleteMedia:      proto.Bool(true),
					MessageTimestamp: proto.Int64(message.Timestamp.UnixMilli()),
				},
			},
		}},
	}, nil
}
