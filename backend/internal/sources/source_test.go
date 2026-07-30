package sources

import (
	"errors"
	"strings"
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

func TestNewURLSourceStartsPendingWithNormalizedIdentity(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	source, err := NewURLSource(
		"11111111-1111-4111-8111-111111111111",
		"PostgreSQL documentation",
		"Read planner notes",
		"https://www.postgresql.org/docs/current/using-explain.html#EXAMPLE",
		"https://www.postgresql.org/docs/current/using-explain.html",
		now,
	)
	if err != nil {
		t.Fatalf("NewURLSource() error = %v", err)
	}
	if source.Kind() != KindURL || source.ProcessingStatus() != ProcessingStatusPending || source.Content() != "" {
		t.Fatalf("URL source kind/status/content = %q / %q / %q", source.Kind(), source.ProcessingStatus(), source.Content())
	}
	if source.OriginalURL() != "https://www.postgresql.org/docs/current/using-explain.html#EXAMPLE" || source.NormalizedURL() != "https://www.postgresql.org/docs/current/using-explain.html" {
		t.Fatalf("URL source addresses = %q / %q", source.OriginalURL(), source.NormalizedURL())
	}
}

func TestNormalizeSourceURLProducesStableHTTPIdentity(t *testing.T) {
	normalized, err := NormalizeSourceURL(" HTTPS://Example.COM:443/path?b=2&a=3&a=1#fragment ")
	if err != nil {
		t.Fatalf("NormalizeSourceURL() error = %v", err)
	}
	if normalized != "https://example.com/path?a=1&a=3&b=2" {
		t.Fatalf("normalized URL = %q", normalized)
	}

	for _, raw := range []string{"ftp://example.com/file", "https://user:secret@example.com/", "https:///missing-host"} {
		if _, err := NormalizeSourceURL(raw); !errors.Is(err, ErrSourceURLInvalid) {
			t.Errorf("NormalizeSourceURL(%q) error = %v, want %v", raw, err, ErrSourceURLInvalid)
		}
	}
}

func TestURLSourceTracksExtractionLifecycle(t *testing.T) {
	createdAt := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	source, _ := NewURLSource("11111111-1111-4111-8111-111111111111", "example.com", "", "https://example.com/", "https://example.com/", createdAt)

	processingAt := createdAt.Add(time.Minute)
	if err := source.MarkExtractionProcessing(processingAt); err != nil {
		t.Fatalf("MarkExtractionProcessing() error = %v", err)
	}
	if source.ProcessingStatus() != ProcessingStatusProcessing || !source.UpdatedAt().Equal(processingAt) {
		t.Fatalf("processing source = status:%q updatedAt:%v", source.ProcessingStatus(), source.UpdatedAt())
	}

	failedAt := processingAt.Add(time.Minute)
	if err := source.RetryExtraction("temporary timeout", failedAt); err != nil {
		t.Fatalf("RetryExtraction() error = %v", err)
	}
	if source.ProcessingStatus() != ProcessingStatusPending || source.FailureMessage() != "temporary timeout" {
		t.Fatalf("retrying source = status:%q failure:%q", source.ProcessingStatus(), source.FailureMessage())
	}

	readyAt := failedAt.Add(time.Minute)
	if err := source.CompleteExtraction(" Extracted title ", "Extracted body", readyAt); err != nil {
		t.Fatalf("CompleteExtraction() error = %v", err)
	}
	if source.ProcessingStatus() != ProcessingStatusReady || source.Title() != "Extracted title" || source.Content() != "Extracted body" || source.FailureMessage() != "" {
		t.Fatalf("ready source = title:%q status:%q content:%q failure:%q", source.Title(), source.ProcessingStatus(), source.Content(), source.FailureMessage())
	}
	if err := source.CompleteExtraction(strings.Repeat("长", 241), "Extracted body", readyAt); err != nil || len([]rune(source.Title())) != 240 {
		t.Fatalf("long extracted title = length:%d error:%v", len([]rune(source.Title())), err)
	}
}
