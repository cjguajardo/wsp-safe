// El paquete filter contiene la política de moderación y se mantiene
// independiente de WhatsApp y de cualquier proveedor de clasificación específico.
package filter

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Kind string

const (
	KindText    Kind = "text"
	KindImage   Kind = "image"
	KindVideo   Kind = "video"
	KindSticker Kind = "sticker"
)

const (
	ReasonIgnoredChat       = "ignored_chat"
	ReasonIgnoredOwnMessage = "ignored_own_message"
	ReasonUnsupported       = "unsupported_content"
	ReasonSafe              = "safe"
	ReasonSexual            = "sexual_content"
	ReasonUncertain         = "uncertain_content"
	ReasonClassifierError   = "classifier_error"
)

type Message struct {
	ID          string
	ChatID      string
	SenderID    string
	FromMe      bool
	Kind        Kind
	MIMEType    string
	Text        string
	Media       []byte
	Timestamp   time.Time
	Unavailable bool
}

type Result struct {
	SexualScore       float64 `json:"sexual_score"`
	SexualMinorsScore float64 `json:"sexual_minors_score"`
	Uncertain         bool    `json:"uncertain"`
}

type Decision struct {
	Delete bool
	Reason string
	Result Result
}

type Config struct {
	TargetChatID    string
	SexualThreshold float64
	DeleteUncertain bool
	DeleteOnError   bool
}

type Classifier interface {
	Classify(context.Context, Message) (Result, error)
}

type Deleter interface {
	DeleteForMe(context.Context, Message) error
}

type AuditEvent struct {
	MessageID string
	ChatID    string
	SenderID  string
	Kind      Kind
	Timestamp time.Time
	Decision  Decision
}

type Auditor interface {
	Record(context.Context, AuditEvent) error
}

type Service struct {
	config     Config
	classifier Classifier
	deleter    Deleter
	auditor    Auditor
}

func New(config Config, classifier Classifier, deleter Deleter, auditor Auditor) (*Service, error) {
	if config.TargetChatID == "" {
		return nil, errors.New("target chat ID is required")
	}
	if config.SexualThreshold <= 0 || config.SexualThreshold > 1 {
		return nil, errors.New("sexual threshold must be greater than zero and at most one")
	}
	if classifier == nil {
		return nil, errors.New("classifier is required")
	}
	if deleter == nil {
		return nil, errors.New("deleter is required")
	}
	return &Service{config: config, classifier: classifier, deleter: deleter, auditor: auditor}, nil
}

func (s *Service) Handle(ctx context.Context, message Message) (Decision, error) {
	if message.ChatID != s.config.TargetChatID {
		return Decision{Reason: ReasonIgnoredChat}, nil
	}
	if message.FromMe {
		return Decision{Reason: ReasonIgnoredOwnMessage}, nil
	}
	if !isSupported(message.Kind) {
		return Decision{Reason: ReasonUnsupported}, nil
	}
	if message.Unavailable {
		decision := Decision{Delete: s.config.DeleteOnError, Reason: ReasonClassifierError}
		if decision.Delete {
			if err := s.deleter.DeleteForMe(ctx, message); err != nil {
				return decision, fmt.Errorf("delete unavailable message for me: %w", err)
			}
		}
		return decision, nil
	}

	result, classifyErr := s.classifier.Classify(ctx, message)
	decision := s.decide(result, classifyErr)
	if decision.Delete {
		if err := s.deleter.DeleteForMe(ctx, message); err != nil {
			return decision, fmt.Errorf("delete message for me: %w", err)
		}
	}

	if s.auditor != nil {
		event := AuditEvent{
			MessageID: message.ID,
			ChatID:    message.ChatID,
			SenderID:  message.SenderID,
			Kind:      message.Kind,
			Timestamp: message.Timestamp,
			Decision:  decision,
		}
		if err := s.auditor.Record(ctx, event); err != nil {
			return decision, fmt.Errorf("record moderation decision: %w", err)
		}
	}
	return decision, nil
}

func (s *Service) decide(result Result, classifyErr error) Decision {
	if classifyErr != nil {
		return Decision{Delete: s.config.DeleteOnError, Reason: ReasonClassifierError}
	}
	if result.SexualScore >= s.config.SexualThreshold || result.SexualMinorsScore >= s.config.SexualThreshold {
		return Decision{Delete: true, Reason: ReasonSexual, Result: result}
	}
	if result.Uncertain && s.config.DeleteUncertain {
		return Decision{Delete: true, Reason: ReasonUncertain, Result: result}
	}
	return Decision{Reason: ReasonSafe, Result: result}
}

func isSupported(kind Kind) bool {
	switch kind {
	case KindText, KindImage, KindVideo, KindSticker:
		return true
	default:
		return false
	}
}
