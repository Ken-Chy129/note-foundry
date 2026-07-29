package notes

import "testing"

func TestExtractNoteLinkTargetsUsesStableNoteURIsAndDeduplicates(t *testing.T) {
	markdown := `[Agent Loop](note:11111111-1111-4111-8111-111111111111)

[Memory](note:22222222-2222-4222-8222-222222222222)
[Agent Loop again](note:11111111-1111-4111-8111-111111111111)
[ordinary](https://example.com)`
	targets := ExtractNoteLinkTargets(markdown)
	if len(targets) != 2 || targets[0] != "11111111-1111-4111-8111-111111111111" || targets[1] != "22222222-2222-4222-8222-222222222222" {
		t.Errorf("targets = %+v", targets)
	}
}

func TestExtractAttachmentTargetsUsesStableAttachmentURI(t *testing.T) {
	markdown := `![diagram](attachment:33333333-3333-4333-8333-333333333333)`
	targets := ExtractAttachmentTargets(markdown)
	if len(targets) != 1 || targets[0] != "33333333-3333-4333-8333-333333333333" {
		t.Errorf("targets = %+v", targets)
	}
}
