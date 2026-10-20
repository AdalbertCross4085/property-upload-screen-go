package screening

import (
	"context"
	"testing"
)

type fakeImages struct {
	url string
}

func (f fakeImages) Resize(context.Context, []byte, string, string) (string, error) {
	return f.url, nil
}

type fakeModerator struct {
	flagged bool
}

func (f fakeModerator) Flagged(context.Context, string, string) (bool, error) {
	return f.flagged, nil
}

func TestScreenMakesPublicationDecision(t *testing.T) {
	tests := []struct {
		name      string
		kind      SubmissionKind
		flagged   bool
		decision  Decision
		wantAsset string
	}{
		{name: "maintenance image is held for review", kind: MaintenanceRequest, flagged: true, decision: Quarantined},
		{name: "tenant document is prepared for publish", kind: TenantDocument, flagged: false, decision: ReadyToPublish, wantAsset: "https://assets.example/unit-7.webp"},
		{name: "inspection reminder is prepared for publish", kind: InspectionReminder, flagged: false, decision: ReadyToPublish, wantAsset: "https://assets.example/unit-7.webp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(fakeImages{url: "https://assets.example/unit-7.webp"}, fakeModerator{flagged: tt.flagged})
			got, err := service.Screen(context.Background(), Submission{
				RequestID: "property-104",
				Kind:      tt.kind,
				Caption:   "Water stain above the east window",
				Filename:  "inspection.jpg",
				Image:     []byte("image bytes"),
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Decision != tt.decision || got.AssetURL != tt.wantAsset {
				t.Fatalf("got decision=%q asset=%q, want decision=%q asset=%q", got.Decision, got.AssetURL, tt.decision, tt.wantAsset)
			}
		})
	}
}
