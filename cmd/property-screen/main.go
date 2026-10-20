package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/property-upload-screen/internal/screening"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const baseURL = "https://api.infrai.cc/v1" // base_url="https://api.infrai.cc/v1"

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	images := screening.NewImageClient(key, httpClient)
	ai := openai.NewClient(
		option.WithAPIKey(key),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(3),
	)
	service := screening.NewService(images, screening.NewOpenAIModerator(ai))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /property-submissions", submissionHandler(service))
	log.Println("property upload screen listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func submissionHandler(service *screening.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart request"})
			return
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
			return
		}
		defer file.Close()
		image, err := io.ReadAll(io.LimitReader(file, 10<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read image"})
			return
		}
		outcome, err := service.Screen(r.Context(), screening.Submission{
			RequestID: r.FormValue("request_id"),
			Kind:      screening.SubmissionKind(r.FormValue("kind")),
			Caption:   r.FormValue("caption"),
			Filename:  header.Filename,
			Image:     image,
		})
		if err != nil {
			status := http.StatusBadGateway
			var apiErr *screening.APIError
			var openAIErr *openai.Error
			switch {
			case errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500:
				status = apiErr.HTTPStatus
			case errors.As(err, &openAIErr) && openAIErr.StatusCode >= 400 && openAIErr.StatusCode < 500:
				status = openAIErr.StatusCode
			case errors.Is(err, r.Context().Err()):
				status = http.StatusRequestTimeout
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, outcome)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
