package sources

import (
	"errors"
	"testing"
	"time"
)

func TestNewManualSourceCreatesReadyInboxItem(t *testing.T) {
	createdAt := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
	source, err := NewManualSource(
		"11111111-1111-4111-8111-111111111111",
		"  PostgreSQL query planning  ",
		"  Compare bitmap and index scans.  ",
		"EXPLAIN ANALYZE output",
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewManualSource() error = %v", err)
	}
	if source.ID() != "11111111-1111-4111-8111-111111111111" || source.Kind() != KindManual {
		t.Fatalf("source identity = id:%q kind:%q", source.ID(), source.Kind())
	}
	if source.Title() != "PostgreSQL query planning" || source.CaptureNote() != "Compare bitmap and index scans." {
		t.Errorf("source copy = title:%q captureNote:%q", source.Title(), source.CaptureNote())
	}
	if source.Content() != "EXPLAIN ANALYZE output" || source.ProcessingStatus() != ProcessingStatusReady {
		t.Errorf("source content/status = %q / %q", source.Content(), source.ProcessingStatus())
	}
	if source.SpaceID() != "" || !source.CreatedAt().Equal(createdAt) || !source.UpdatedAt().Equal(createdAt) {
		t.Errorf("source inbox/timestamps = space:%q created:%v updated:%v", source.SpaceID(), source.CreatedAt(), source.UpdatedAt())
	}
}

func TestNewManualSourceRequiresIdentityAndTitle(t *testing.T) {
	now := time.Now()
	if _, err := NewManualSource("", "Source", "", "", now); !errors.Is(err, ErrSourceIDRequired) {
		t.Fatalf("missing id error = %v, want %v", err, ErrSourceIDRequired)
	}
	if _, err := NewManualSource("11111111-1111-4111-8111-111111111111", "  ", "", "", now); !errors.Is(err, ErrSourceTitleRequired) {
		t.Fatalf("missing title error = %v, want %v", err, ErrSourceTitleRequired)
	}
}

func TestSourceOrganizeKeepsStableIdentityAndCanReturnToInbox(t *testing.T) {
	createdAt := time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
	organizedAt := createdAt.Add(time.Hour)
	returnedAt := organizedAt.Add(time.Hour)
	source, _ := NewManualSource("11111111-1111-4111-8111-111111111111", "Source", "", "", createdAt)

	source.Organize(" 22222222-2222-4222-8222-222222222222 ", organizedAt)
	if source.ID() != "11111111-1111-4111-8111-111111111111" || source.SpaceID() != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("organized source = id:%q space:%q", source.ID(), source.SpaceID())
	}
	if !source.UpdatedAt().Equal(organizedAt) {
		t.Fatalf("organized updatedAt = %v, want %v", source.UpdatedAt(), organizedAt)
	}

	source.Organize("", returnedAt)
	if source.SpaceID() != "" || !source.UpdatedAt().Equal(returnedAt) {
		t.Fatalf("returned source = space:%q updatedAt:%v", source.SpaceID(), source.UpdatedAt())
	}
}
