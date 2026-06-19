package pr

import "testing"

func TestDedupeReviewing(t *testing.T) {
	authored := []PR{{URL: "a"}, {URL: "b"}}
	reviewing := []PR{{URL: "b"}, {URL: "c"}}

	got := DedupeReviewing(authored, reviewing)
	if len(got) != 1 || got[0].URL != "c" {
		t.Fatalf("DedupeReviewing = %+v, want only [c]", got)
	}

	// No overlap leaves reviewing intact.
	got = DedupeReviewing([]PR{{URL: "x"}}, []PR{{URL: "y"}, {URL: "z"}})
	if len(got) != 2 {
		t.Errorf("expected 2 retained, got %d", len(got))
	}
}
