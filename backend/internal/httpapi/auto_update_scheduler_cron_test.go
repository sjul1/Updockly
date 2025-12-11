package httpapi

import "testing"

func TestCronMatchesSimple(t *testing.T) {
	// 0 0 * * * should match midnight
	if !cronMatches("0 0 * * *", mustTime("2024-01-01T00:00:00Z")) {
		t.Fatalf("expected cron to match midnight")
	}
	if cronMatches("0 0 * * *", mustTime("2024-01-01T01:00:00Z")) {
		t.Fatalf("did not expect cron to match 1am")
	}
}

func TestCronMatchesStepAndList(t *testing.T) {
	if !cronMatches("*/15 9-17 * * 1,2", mustTime("2024-01-02T09:45:00Z")) {
		t.Fatalf("expected cron to match step/list")
	}
	if cronMatches("*/15 9-17 * * 1,2", mustTime("2024-01-03T09:45:00Z")) {
		t.Fatalf("did not expect cron to match wrong weekday")
	}
}
