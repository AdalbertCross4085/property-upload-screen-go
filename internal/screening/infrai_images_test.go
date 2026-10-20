package screening

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestResizeSendsJSON(t *testing.T) {
	client := NewImageClient("test-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		var body struct {
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
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Image.Base64 != "/9j/2w==" {
			t.Fatalf("image.base64 = %q", body.Image.Base64)
		}
		if body.Width != 1600 || body.Height != 1200 || body.Fit != "inside" || body.Enlarge || body.Format != "webp" || body.IdempotencyKey != "property-104-resize" || !body.Store {
			t.Fatalf("unexpected resize body: %+v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"data":{"url":"https://assets.example/resized.webp"}}`)),
		}, nil
	})})

	got, err := client.Resize(context.Background(), []byte{0xff, 0xd8, 0xff, 0xdb}, "photo.jpg", "property-104")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://assets.example/resized.webp" {
		t.Fatalf("Resize() = %q", got)
	}
}
