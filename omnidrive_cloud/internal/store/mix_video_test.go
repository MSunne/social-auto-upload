package store

import (
	"testing"
	"time"
)

func TestScanMixVideoTaskIncludesAccountBindingFields(t *testing.T) {
	now := time.Now().UTC()
	runAt := now.Add(30 * time.Minute)
	sourcePayload := []byte(`[{"storageKey":"video-1","publicUrl":"https://example.com/video-1.mp4","fileName":"video-1.mp4","mimeType":"video/mp4"}]`)
	refAudioPayload := []byte(`{"storageKey":"audio-1","publicUrl":"https://example.com/audio-1.m4a","fileName":"audio-1.m4a","mimeType":"audio/m4a"}`)
	schedulePayload := []byte(`{"scheduleKey":"slot-1"}`)

	task, err := scanMixVideoTask(scanRowFunc(func(dest ...any) error {
		if len(dest) != 36 {
			t.Fatalf("scanMixVideoTask destination count = %d, want %d", len(dest), 36)
		}
		*(dest[0].(*string)) = "task-1"
		*(dest[1].(*string)) = "user-1"
		*(dest[2].(*string)) = "account_skill_binding"
		*(dest[3].(*string)) = "scheduled"
		*(dest[5].(*[]byte)) = sourcePayload
		*(dest[6].(*[]byte)) = refAudioPayload
		*(dest[8].(*string)) = "脚本文案"
		*(dest[9].(**string)) = stringPtr("device-1")
		*(dest[10].(**string)) = stringPtr("skill-1")
		*(dest[11].(**string)) = stringPtr("account-1")
		*(dest[12].(**string)) = stringPtr("抖音")
		*(dest[13].(**string)) = stringPtr("账号A")
		*(dest[14].(**time.Time)) = &runAt
		*(dest[15].(*[]byte)) = schedulePayload
		*(dest[16].(**string)) = stringPtr("publish-task-1")
		*(dest[17].(*int)) = 15
		*(dest[18].(*int64)) = 0
		*(dest[19].(*int64)) = 1500
		*(dest[23].(*string)) = "pending"
		*(dest[31].(*time.Time)) = now
		*(dest[32].(*time.Time)) = now
		return nil
	}))
	if err != nil {
		t.Fatalf("scanMixVideoTask returned error: %v", err)
	}

	if task.DeviceID == nil || *task.DeviceID != "device-1" {
		t.Fatalf("DeviceID = %#v, want %q", task.DeviceID, "device-1")
	}
	if task.SkillID == nil || *task.SkillID != "skill-1" {
		t.Fatalf("SkillID = %#v, want %q", task.SkillID, "skill-1")
	}
	if task.AccountID == nil || *task.AccountID != "account-1" {
		t.Fatalf("AccountID = %#v, want %q", task.AccountID, "account-1")
	}
	if task.Platform == nil || *task.Platform != "抖音" {
		t.Fatalf("Platform = %#v, want %q", task.Platform, "抖音")
	}
	if task.AccountName == nil || *task.AccountName != "账号A" {
		t.Fatalf("AccountName = %#v, want %q", task.AccountName, "账号A")
	}
	if task.RunAt == nil || !task.RunAt.Equal(runAt) {
		t.Fatalf("RunAt = %#v, want %v", task.RunAt, runAt)
	}
	if string(task.SchedulePayload) != string(schedulePayload) {
		t.Fatalf("SchedulePayload = %s, want %s", string(task.SchedulePayload), string(schedulePayload))
	}
	if task.LocalPublishTaskID == nil || *task.LocalPublishTaskID != "publish-task-1" {
		t.Fatalf("LocalPublishTaskID = %#v, want %q", task.LocalPublishTaskID, "publish-task-1")
	}
}

type scanRowFunc func(dest ...any) error

func (f scanRowFunc) Scan(dest ...any) error {
	return f(dest...)
}
