package classifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cgc/wsp-safe/internal/filter"
)

func TestHTTPClassifier(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q, want bearer token", got)
		}
		var request classifyRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.MediaBase64 == "" {
			t.Error("media_base64 is empty")
		}
		if request.SenderID != "56911111111@s.whatsapp.net" {
			t.Errorf("sender_id = %q", request.SenderID)
		}
		return response(http.StatusOK, `{"sexual_score":0.82,"sexual_minors_score":0.01,"uncertain":false}`), nil
	})}

	classifier, err := NewHTTP("https://classifier.test/v1/classify", "secret", client)
	if err != nil {
		t.Fatalf("NewHTTP() error = %v", err)
	}
	result, err := classifier.Classify(context.Background(), filter.Message{
		ID:       "message-id",
		SenderID: "56911111111@s.whatsapp.net",
		Kind:     filter.KindImage,
		MIMEType: "image/jpeg",
		Media:    []byte("image bytes"),
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if result.SexualScore != 0.82 {
		t.Errorf("sexual score = %v, want 0.82", result.SexualScore)
	}
}

func TestHTTPClassifierRejectsInvalidResponse(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"sexual_score":1.4}`), nil
	})}
	client, err := NewHTTP("https://classifier.test/v1/classify", "", httpClient)
	if err != nil {
		t.Fatalf("NewHTTP() error = %v", err)
	}
	if _, err = client.Classify(context.Background(), filter.Message{Kind: filter.KindText, Text: "hello"}); err == nil {
		t.Fatal("Classify() error = nil, want invalid score error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
