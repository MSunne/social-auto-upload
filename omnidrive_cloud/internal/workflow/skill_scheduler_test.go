package workflow

import (
	"testing"
	"time"
)

func TestScheduledSkillGenerationTime(t *testing.T) {
	publishAt := time.Date(2026, 3, 19, 12, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	got := scheduledSkillGenerationTime(publishAt)

	want := time.Date(2026, 3, 19, 4, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("scheduledSkillGenerationTime() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}
