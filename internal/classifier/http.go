package classifier

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cgc/wsp-safe/internal/filter"
)

const maxResponseBytes = 64 << 10

type classifyRequest struct {
	MessageID   string      `json:"message_id"`
	Kind        filter.Kind `json:"kind"`
	MIMEType    string      `json:"mime_type,omitempty"`
	Text        string      `json:"text,omitempty"`
	MediaBase64 string      `json:"media_base64,omitempty"`
}

type HTTP struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewHTTP(endpoint, token string, client *http.Client) (*HTTP, error) {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("classifier endpoint must be an absolute HTTP(S) URL")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTP{endpoint: endpoint, token: token, client: client}, nil
}

func (h *HTTP) Classify(ctx context.Context, message filter.Message) (filter.Result, error) {
	payload := classifyRequest{
		MessageID: message.ID,
		Kind:      message.Kind,
		MIMEType:  message.MIMEType,
		Text:      message.Text,
	}
	if len(message.Media) > 0 {
		payload.MediaBase64 = base64.StdEncoding.EncodeToString(message.Media)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return filter.Result{}, fmt.Errorf("encode classifier request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		return filter.Result{}, fmt.Errorf("create classifier request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	response, err := h.client.Do(req)
	if err != nil {
		return filter.Result{}, fmt.Errorf("call classifier: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		return filter.Result{}, fmt.Errorf("classifier returned HTTP %d", response.StatusCode)
	}

	var result filter.Result
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(&result); err != nil {
		return filter.Result{}, fmt.Errorf("decode classifier response: %w", err)
	}
	if !validScore(result.SexualScore) || !validScore(result.SexualMinorsScore) {
		return filter.Result{}, errors.New("classifier returned a score outside the range 0..1")
	}
	return result, nil
}

func validScore(score float64) bool {
	return score >= 0 && score <= 1
}
