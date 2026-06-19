package pr

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	b := NewBackoff(45 * time.Second)

	if b.Delay() != 45*time.Second {
		t.Fatalf("initial delay = %v, want 45s", b.Delay())
	}
	if b.Failures() != 0 {
		t.Fatalf("initial failures = %d", b.Failures())
	}

	want := []time.Duration{5, 10, 20, 40, 60, 60}
	for i, w := range want {
		b.RecordFailure()
		if got := b.Delay(); got != w*time.Second {
			t.Errorf("after %d failures: delay = %v, want %v", i+1, got, w*time.Second)
		}
	}
	if b.Failures() != len(want) {
		t.Errorf("failures = %d, want %d", b.Failures(), len(want))
	}

	b.RecordSuccess()
	if b.Delay() != 45*time.Second {
		t.Errorf("after success: delay = %v, want 45s", b.Delay())
	}
	if b.Failures() != 0 {
		t.Errorf("after success: failures = %d, want 0", b.Failures())
	}
}
