package filter

import (
	"context"
	"errors"
	"testing"
)

func TestServiceHandle(t *testing.T) {
	tests := []struct {
		name           string
		message        Message
		result         Result
		classifierErr  error
		deleteOnError  bool
		wantClassified bool
		wantDeleted    bool
		wantReason     string
	}{
		{
			name:       "ignores another group",
			message:    Message{ID: "m1", ChatID: "other@g.us", Kind: KindImage},
			wantReason: ReasonIgnoredChat,
		},
		{
			name:       "ignores own messages",
			message:    Message{ID: "m2", ChatID: "target@g.us", FromMe: true, Kind: KindImage},
			wantReason: ReasonIgnoredOwnMessage,
		},
		{
			name:           "keeps safe content",
			message:        Message{ID: "m3", ChatID: "target@g.us", Kind: KindImage},
			result:         Result{SexualScore: 0.04},
			wantClassified: true,
			wantReason:     ReasonSafe,
		},
		{
			name:           "deletes sexual content at threshold",
			message:        Message{ID: "m4", ChatID: "target@g.us", Kind: KindVideo},
			result:         Result{SexualScore: 0.25},
			wantClassified: true,
			wantDeleted:    true,
			wantReason:     ReasonSexual,
		},
		{
			name:           "deletes uncertain content",
			message:        Message{ID: "m5", ChatID: "target@g.us", Kind: KindSticker},
			result:         Result{Uncertain: true},
			wantClassified: true,
			wantDeleted:    true,
			wantReason:     ReasonUncertain,
		},
		{
			name:           "fails closed when classifier is unavailable",
			message:        Message{ID: "m6", ChatID: "target@g.us", Kind: KindImage},
			classifierErr:  errors.New("classifier unavailable"),
			deleteOnError:  true,
			wantClassified: true,
			wantDeleted:    true,
			wantReason:     ReasonClassifierError,
		},
		{
			name:           "fails closed when media could not be loaded",
			message:        Message{ID: "m7", ChatID: "target@g.us", Kind: KindImage, Unavailable: true},
			deleteOnError:  true,
			wantClassified: false,
			wantDeleted:    true,
			wantReason:     ReasonClassifierError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier := &fakeClassifier{result: tt.result, err: tt.classifierErr}
			deleter := &fakeDeleter{}
			service, err := New(Config{
				TargetChatID:    "target@g.us",
				SexualThreshold: 0.25,
				DeleteUncertain: true,
				DeleteOnError:   tt.deleteOnError,
			}, classifier, deleter, nil)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			decision, err := service.Handle(context.Background(), tt.message)
			if err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if got := classifier.calls > 0; got != tt.wantClassified {
				t.Errorf("classified = %v, want %v", got, tt.wantClassified)
			}
			if got := len(deleter.deleted) == 1; got != tt.wantDeleted {
				t.Errorf("deleted = %v, want %v", got, tt.wantDeleted)
			}
			if decision.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", decision.Reason, tt.wantReason)
			}
		})
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{name: "missing target chat", config: Config{SexualThreshold: 0.25}},
		{name: "zero threshold", config: Config{TargetChatID: "target@g.us"}},
		{name: "threshold above one", config: Config{TargetChatID: "target@g.us", SexualThreshold: 1.01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config, &fakeClassifier{}, &fakeDeleter{}, nil)
			if err == nil {
				t.Fatal("New() error = nil, want validation error")
			}
		})
	}
}

type fakeClassifier struct {
	result Result
	err    error
	calls  int
}

func (f *fakeClassifier) Classify(context.Context, Message) (Result, error) {
	f.calls++
	return f.result, f.err
}

type fakeDeleter struct {
	deleted []Message
	err     error
}

func (f *fakeDeleter) DeleteForMe(_ context.Context, message Message) error {
	f.deleted = append(f.deleted, message)
	return f.err
}
