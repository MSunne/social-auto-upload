package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"omnidrive_cloud/internal/domain"
)

func TestScheduledSkillGenerationTime(t *testing.T) {
	publishAt := time.Date(2026, 3, 19, 12, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	got := scheduledSkillGenerationTime(publishAt)

	want := time.Date(2026, 3, 19, 4, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("scheduledSkillGenerationTime() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestNextScheduledSkillRunAt(t *testing.T) {
	publishAt := time.Date(2026, 3, 20, 13, 34, 45, 0, time.UTC)

	t.Run("one_off_skill_clears_next_run", func(t *testing.T) {
		skill := domain.ProductSkill{RepeatDaily: false}
		if got := nextScheduledSkillRunAt(skill, publishAt); got != nil {
			t.Fatalf("expected nil next run for one-off skill, got %s", got.Format(time.RFC3339))
		}
	})

	t.Run("daily_skill_advances_by_one_day", func(t *testing.T) {
		skill := domain.ProductSkill{RepeatDaily: true}
		got := nextScheduledSkillRunAt(skill, publishAt)
		if got == nil {
			t.Fatalf("expected next run for daily skill")
		}
		want := publishAt.Add(24 * time.Hour)
		if !got.Equal(want) {
			t.Fatalf("nextScheduledSkillRunAt() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})
}

func TestNormalizeAccountSkillTimeOfDay(t *testing.T) {
	got, err := NormalizeAccountSkillTimeOfDay("09:15")
	if err != nil {
		t.Fatalf("NormalizeAccountSkillTimeOfDay() returned error: %v", err)
	}
	if got != "09:15:00" {
		t.Fatalf("NormalizeAccountSkillTimeOfDay() = %q, want %q", got, "09:15:00")
	}
}

func TestNextAccountSkillPublishAt(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 3, 21, 10, 0, 0, 0, location)

	t.Run("same_day_when_time_is_ahead", func(t *testing.T) {
		got, err := NextAccountSkillPublishAt("12:30", "Asia/Shanghai", now)
		if err != nil {
			t.Fatalf("NextAccountSkillPublishAt() returned error: %v", err)
		}
		want := time.Date(2026, 3, 21, 4, 30, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("NextAccountSkillPublishAt() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})

	t.Run("next_day_when_time_has_passed", func(t *testing.T) {
		got, err := NextAccountSkillPublishAt("09:00", "Asia/Shanghai", now)
		if err != nil {
			t.Fatalf("NextAccountSkillPublishAt() returned error: %v", err)
		}
		want := time.Date(2026, 3, 22, 1, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("NextAccountSkillPublishAt() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})
}

func TestScheduledAccountSkillGenerationTime(t *testing.T) {
	publishAt := time.Date(2026, 3, 21, 7, 22, 0, 0, time.UTC)

	t.Run("no_lead_generates_at_publish_time", func(t *testing.T) {
		got := ScheduledAccountSkillGenerationTime(publishAt, 0)
		if !got.Equal(publishAt) {
			t.Fatalf("ScheduledAccountSkillGenerationTime() = %s, want %s", got.Format(time.RFC3339), publishAt.Format(time.RFC3339))
		}
	})

	t.Run("custom_lead_generates_before_publish_time", func(t *testing.T) {
		got := ScheduledAccountSkillGenerationTime(publishAt, 10)
		want := publishAt.Add(-10 * time.Minute)
		if !got.Equal(want) {
			t.Fatalf("ScheduledAccountSkillGenerationTime() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})
}

func TestParseAccountSkillScheduleConfigInfersLegacyGenerationLead(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"runAt":     "2026-03-21T06:52:00Z",
		"publishAt": "2026-03-21T07:22:00Z",
		"scheduleConfig": map[string]any{
			"scheduleKey": "legacy-slot",
			"timeOfDay":   "15:22:00",
			"repeatDaily": true,
			"timezone":    "Asia/Shanghai",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	config, ok := ParseAccountSkillScheduleConfig(raw)
	if !ok || config == nil {
		t.Fatalf("ParseAccountSkillScheduleConfig() returned nil config")
	}
	if config.GenerationLeadMinutes != 30 {
		t.Fatalf("GenerationLeadMinutes = %d, want 30", config.GenerationLeadMinutes)
	}
}

func TestBuildDefaultAccountSkillScheduleKeyIsDeterministic(t *testing.T) {
	config := AccountSkillScheduleConfig{
		TimeOfDay:             "09:15:00",
		RepeatDaily:           true,
		Timezone:              "Asia/Shanghai",
		GenerationLeadMinutes: 20,
	}

	first := BuildDefaultAccountSkillScheduleKey("skill-1", "account-1", config)
	second := BuildDefaultAccountSkillScheduleKey("skill-1", "account-1", config)
	other := BuildDefaultAccountSkillScheduleKey("skill-1", "account-2", config)

	if first == "" {
		t.Fatalf("expected non-empty schedule key")
	}
	if first != second {
		t.Fatalf("expected deterministic key, got %q and %q", first, second)
	}
	if first == other {
		t.Fatalf("expected account-specific key, got identical values %q", first)
	}
}

func TestResolveSkillVideoGenerationOptionsUsesModelDefaults(t *testing.T) {
	model := &domain.AIModel{
		ModelName:                 "veo-3.1-fast-fl",
		VideoSupportedResolutions: []string{"720x1280", "1280x720"},
		VideoSupportedDurations:   []string{"12s", "8s"},
	}

	got := resolveSkillVideoGenerationOptions(domain.ProductSkill{}, model)

	if got.Resolution != "720x1280" {
		t.Fatalf("Resolution = %q, want %q", got.Resolution, "720x1280")
	}
	if got.AspectRatio != "9:16" {
		t.Fatalf("AspectRatio = %q, want %q", got.AspectRatio, "9:16")
	}
	if got.DurationSeconds == nil || *got.DurationSeconds != 12 {
		t.Fatalf("DurationSeconds = %#v, want 12", got.DurationSeconds)
	}
}

func TestResolveSkillVideoGenerationOptionsAllowsSkillOverrides(t *testing.T) {
	referencePayload, err := json.Marshal(map[string]any{
		"videoSettings": map[string]any{
			"aspectRatio":     "16:9",
			"resolution":      "1280x720",
			"durationSeconds": 8,
		},
	})
	if err != nil {
		t.Fatalf("marshal reference payload: %v", err)
	}

	got := resolveSkillVideoGenerationOptions(domain.ProductSkill{ReferencePayload: referencePayload}, &domain.AIModel{
		ModelName:                 "veo-3.1-fast-fl",
		VideoSupportedResolutions: []string{"720x1280"},
		VideoSupportedDurations:   []string{"12s"},
	})

	if got.Resolution != "1280x720" {
		t.Fatalf("Resolution = %q, want %q", got.Resolution, "1280x720")
	}
	if got.AspectRatio != "16:9" {
		t.Fatalf("AspectRatio = %q, want %q", got.AspectRatio, "16:9")
	}
	if got.DurationSeconds == nil || *got.DurationSeconds != 8 {
		t.Fatalf("DurationSeconds = %#v, want 8", got.DurationSeconds)
	}
}
