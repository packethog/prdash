package main

import (
	"testing"
	"time"
)

func TestParseFlagsDefaults(t *testing.T) {
	c, err := parseFlags(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.interval != 45*time.Second || c.limit != 50 {
		t.Errorf("defaults = %+v", c)
	}
}

func TestParseFlagsOverrides(t *testing.T) {
	c, err := parseFlags([]string{"--interval", "30", "--limit", "10"})
	if err != nil {
		t.Fatal(err)
	}
	if c.interval != 30*time.Second || c.limit != 10 {
		t.Errorf("overrides = %+v", c)
	}
}

func TestParseFlagsClampsLowValues(t *testing.T) {
	c, err := parseFlags([]string{"--interval", "1", "--limit", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if c.interval != 5*time.Second || c.limit != 1 {
		t.Errorf("clamped = %+v", c)
	}
}

func TestParseFlagsClampsLimitToMax(t *testing.T) {
	c, err := parseFlags([]string{"--limit", "500"})
	if err != nil {
		t.Fatal(err)
	}
	if c.limit != 100 {
		t.Errorf("limit = %d, want clamped to 100", c.limit)
	}
}
