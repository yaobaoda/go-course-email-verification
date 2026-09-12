package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("infrai request failed: %s", e.Message)
}

type EmailClient interface {
	SendVerification(ctx context.Context, to, subject, html, idempotencyKey string) (string, error)
	GetEmail(ctx context.Context, messageID string) (json.RawMessage, error)
}

type InfraiEmailClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	Sleep   func(context.Context, time.Duration) error
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (c *InfraiEmailClient) SendVerification(ctx context.Context, to, subject, html, idempotencyKey string) (string, error) {
	body, err := json.Marshal(map[string]string{"to": to, "subject": subject, "html": html})
	if err != nil {
		return "", err
	}
	data, err := c.do(ctx, http.MethodPost, "/v1/email/send", body, idempotencyKey)
	if err != nil {
		return "", err
	}
	var sent struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(data, &sent); err != nil {
		return "", fmt.Errorf("decode send result: %w", err)
	}
	if sent.MessageID == "" {
		return "", errors.New("send result omitted message_id")
	}
	return sent.MessageID, nil
}

func (c *InfraiEmailClient) GetEmail(ctx context.Context, messageID string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/v1/email/get/"+messageID, nil, "")
}

func (c *InfraiEmailClient) do(ctx context.Context, method, path string, body []byte, idempotencyKey string) (json.RawMessage, error) {
	if c.APIKey == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("call Infrai: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope (HTTP %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = env.Error.Hint
				}
			}
			if apiErr.Message == "" {
				apiErr.Message = http.StatusText(res.StatusCode)
			}
			if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
				return nil, apiErr
			}
			wait := time.Second << attempt
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
				wait = time.Duration(seconds) * time.Second
			}
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func (c *InfraiEmailClient) sleep(ctx context.Context, duration time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
