package screening

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
)

type SubmissionKind string

const (
	MaintenanceRequest SubmissionKind = "maintenance_request"
	TenantDocument     SubmissionKind = "tenant_document"
	InspectionReminder SubmissionKind = "inspection_reminder"
)

type Submission struct {
	RequestID string
	Kind      SubmissionKind
	Caption   string
	Filename  string
	Image     []byte
}

type Decision string

const (
	ReadyToPublish Decision = "ready_to_publish"
	Quarantined    Decision = "quarantined"
)

type Outcome struct {
	RequestID string   `json:"request_id"`
	Kind      string   `json:"kind"`
	Decision  Decision `json:"decision"`
	AssetURL  string   `json:"asset_url,omitempty"`
}

type ImageTransformer interface {
	Resize(context.Context, []byte, string, string) (string, error)
}

type Moderator interface {
	Flagged(context.Context, string, string) (bool, error)
}

type Service struct {
	images    ImageTransformer
	moderator Moderator
}

func NewService(images ImageTransformer, moderator Moderator) *Service {
	return &Service{images: images, moderator: moderator}
}

func (s *Service) Screen(ctx context.Context, input Submission) (Outcome, error) {
	if err := validate(input); err != nil {
		return Outcome{}, err
	}
	assetURL, err := s.images.Resize(ctx, input.Image, input.Filename, input.RequestID)
	if err != nil {
		return Outcome{}, err
	}
	flagged, err := s.moderator.Flagged(ctx, input.Caption, assetURL)
	if err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{RequestID: input.RequestID, Kind: string(input.Kind), Decision: ReadyToPublish, AssetURL: assetURL}
	if flagged {
		outcome.Decision = Quarantined
		outcome.AssetURL = ""
	}
	return outcome, nil
}

func validate(input Submission) error {
	if input.RequestID == "" || input.Caption == "" || input.Filename == "" || len(input.Image) == 0 {
		return fmt.Errorf("request_id, kind, caption, filename, and image are required")
	}
	switch input.Kind {
	case MaintenanceRequest, TenantDocument, InspectionReminder:
		return nil
	default:
		return fmt.Errorf("unknown submission kind %q", input.Kind)
	}
}

type OpenAIModerator struct {
	client openai.Client
}

func NewOpenAIModerator(client openai.Client) *OpenAIModerator {
	return &OpenAIModerator{client: client}
}

func (m *OpenAIModerator) Flagged(ctx context.Context, caption, assetURL string) (bool, error) {
	result, err := m.client.Moderations.New(ctx, openai.ModerationNewParams{
		Model: openai.ModerationModelOmniModerationLatest,
		Input: openai.ModerationNewParamsInputUnion{OfModerationMultiModalArray: []openai.ModerationMultiModalInputUnionParam{
			openai.ModerationMultiModalInputParamOfText(caption),
			openai.ModerationMultiModalInputParamOfImageURL(openai.ModerationImageURLInputImageURLParam{URL: assetURL}),
		}},
	})
	if err != nil {
		return false, fmt.Errorf("moderate property submission: %w", err)
	}
	for _, item := range result.Results {
		if item.Flagged {
			return true, nil
		}
	}
	return false, nil
}
