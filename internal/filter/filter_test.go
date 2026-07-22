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
			name:           "classifies messages from any chat",
			message:        Message{ID: "m1", ChatID: "person@s.whatsapp.net", Kind: KindImage},
			result:         Result{SexualScore: 0.04},
			wantClassified: true,
			wantReason:     ReasonSafe,
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
				SexualThreshold: 0.25,
				DeleteUncertain: true,
				DeleteOnError:   tt.deleteOnError,
			}, classifier, deleter, nil, nil)
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
		{name: "zero threshold", config: Config{}},
		{name: "threshold above one", config: Config{SexualThreshold: 1.01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config, &fakeClassifier{}, &fakeDeleter{}, nil, nil)
			if err == nil {
				t.Fatal("New() error = nil, want validation error")
			}
		})
	}
}

func TestServiceArchivesDeletedContentBeforeDeleting(t *testing.T) {
	sequence := make([]string, 0, 2)
	archiver := &fakeArchiver{sequence: &sequence}
	deleter := &fakeDeleter{sequence: &sequence}
	service, err := New(Config{
		SexualThreshold: 0.25,
		DeleteUncertain: true,
		DeleteOnError:   true,
	}, &fakeClassifier{result: Result{SexualScore: 0.9}}, deleter, archiver, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	message := Message{ID: "m-archivo", ChatID: "persona@s.whatsapp.net", Kind: KindImage, Media: []byte("privado")}
	decision, err := service.Handle(context.Background(), message)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !decision.Delete || len(archiver.messages) != 1 {
		t.Fatalf("decisión = %+v, archivos = %d", decision, len(archiver.messages))
	}
	if got := sequence; len(got) != 2 || got[0] != "archive" || got[1] != "delete" {
		t.Errorf("secuencia = %v", got)
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
	deleted  []Message
	err      error
	sequence *[]string
}

func (f *fakeDeleter) DeleteForMe(_ context.Context, message Message) error {
	if f.sequence != nil {
		*f.sequence = append(*f.sequence, "delete")
	}
	f.deleted = append(f.deleted, message)
	return f.err
}

type fakeArchiver struct {
	messages []Message
	sequence *[]string
}

func (f *fakeArchiver) Archive(_ context.Context, message Message, _ Decision) error {
	if f.sequence != nil {
		*f.sequence = append(*f.sequence, "archive")
	}
	f.messages = append(f.messages, message)
	return nil
}
