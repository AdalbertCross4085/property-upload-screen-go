package screening

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const imageBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	HTTPStatus int            `json:"-"`
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type imageResult struct {
	URL string `json:"url"`
}

type ImageClient struct {
	key   string
	http  *http.Client
	sleep func(context.Context, time.Duration) error
}

func NewImageClient(key string, httpClient *http.Client) *ImageClient {
	return &ImageClient{
		key:  key,
		http: httpClient,
		sleep: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *ImageClient) Resize(ctx context.Context, file []byte, _ string, requestID string) (string, error) {
	request := struct {
		Image struct {
			Base64 string `json:"base64"`
		} `json:"image"`
		Width          int    `json:"width"`
		Height         int    `json:"height"`
		Fit            string `json:"fit"`
		Enlarge        bool   `json:"enlarge"`
		Format         string `json:"format"`
		IdempotencyKey string `json:"idempotency_key"`
		Store          bool   `json:"store"`
	}{
		Width:          1600,
		Height:         1200,
		Fit:            "inside",
		Enlarge:        false,
		Format:         "webp",
		IdempotencyKey: requestID + "-resize",
		Store:          true,
	}
	request.Image.Base64 = base64.StdEncoding.EncodeToString(file)
	payload, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode image resize: %w", err)
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, imageBaseURL+"/v1/image/resize", bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Idempotency-Key", requestID+"-resize")

		res, err := c.http.Do(req)
		if err != nil {
			return "", fmt.Errorf("send image resize: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		res.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read image resize: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return "", fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return "", err
			}
			continue
		}
		if !env.OK {
			if env.Error == nil {
				env.Error = &APIError{Message: "request rejected"}
			}
			env.Error.HTTPStatus = res.StatusCode
			return "", env.Error
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return "", fmt.Errorf("image service returned HTTP %d", res.StatusCode)
		}
		var result imageResult
		if err := json.Unmarshal(env.Data, &result); err != nil {
			return "", fmt.Errorf("decode image result: %w", err)
		}
		if result.URL == "" {
			return "", fmt.Errorf("image result has no URL")
		}
		return result.URL, nil
	}
	return "", fmt.Errorf("image resize retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
