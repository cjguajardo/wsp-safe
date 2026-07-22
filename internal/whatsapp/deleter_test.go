package whatsapp

import (
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/appstate"

	"github.com/cgc/wsp-safe/internal/filter"
)

func TestBuildDeleteForMePatch(t *testing.T) {
	timestamp := time.Unix(1_750_000_000, 0)
	patch, err := BuildDeleteForMePatch(filter.Message{
		ID:        "message-id",
		ChatID:    "120363000000000000@g.us",
		SenderID:  "56911111111@s.whatsapp.net",
		Timestamp: timestamp,
	})
	if err != nil {
		t.Fatalf("BuildDeleteForMePatch() error = %v", err)
	}

	if patch.Type != appstate.WAPatchRegularHigh {
		t.Errorf("patch type = %q, want %q", patch.Type, appstate.WAPatchRegularHigh)
	}
	if len(patch.Mutations) != 1 {
		t.Fatalf("mutations = %d, want 1", len(patch.Mutations))
	}
	mutation := patch.Mutations[0]
	wantIndex := []string{
		appstate.IndexDeleteMessageForMe,
		"120363000000000000@g.us",
		"message-id",
		"0",
		"56911111111@s.whatsapp.net",
	}
	for i, want := range wantIndex {
		if mutation.Index[i] != want {
			t.Errorf("index[%d] = %q, want %q", i, mutation.Index[i], want)
		}
	}
	if mutation.Version != 2 {
		t.Errorf("version = %d, want 2", mutation.Version)
	}
	action := mutation.Value.GetDeleteMessageForMeAction()
	if action == nil {
		t.Fatal("delete action = nil")
	}
	if !action.GetDeleteMedia() {
		t.Error("delete media = false, want true")
	}
	if action.GetMessageTimestamp() != timestamp.UnixMilli() {
		t.Errorf("timestamp = %d, want %d", action.GetMessageTimestamp(), timestamp.UnixMilli())
	}
}

func TestBuildDeleteForMePatchRejectsIncompleteMessage(t *testing.T) {
	tests := []struct {
		name    string
		message filter.Message
	}{
		{name: "missing message ID", message: filter.Message{ChatID: "group@g.us", SenderID: "sender@s.whatsapp.net", Timestamp: time.Now()}},
		{name: "missing chat ID", message: filter.Message{ID: "id", SenderID: "sender@s.whatsapp.net", Timestamp: time.Now()}},
		{name: "missing sender ID", message: filter.Message{ID: "id", ChatID: "group@g.us", Timestamp: time.Now()}},
		{name: "missing timestamp", message: filter.Message{ID: "id", ChatID: "group@g.us", SenderID: "sender@s.whatsapp.net"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildDeleteForMePatch(tt.message); err == nil {
				t.Fatal("BuildDeleteForMePatch() error = nil, want validation error")
			}
		})
	}
}

func TestDeleterSendsPatch(t *testing.T) {
	sender := &fakeAppStateSender{}
	deleter := NewDeleter(sender)
	err := deleter.DeleteForMe(context.Background(), filter.Message{
		ID:        "id",
		ChatID:    "group@g.us",
		SenderID:  "sender@s.whatsapp.net",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("DeleteForMe() error = %v", err)
	}
	if sender.calls != 1 {
		t.Errorf("SendAppState() calls = %d, want 1", sender.calls)
	}
}

type fakeAppStateSender struct {
	calls int
}

func (f *fakeAppStateSender) SendAppState(context.Context, appstate.PatchInfo) error {
	f.calls++
	return nil
}
