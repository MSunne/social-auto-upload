package store

import (
	"reflect"
	"testing"
	"time"
)

func TestScanAdminUserRowIncludesPhone(t *testing.T) {
	now := time.Now().UTC()
	notes := "vip user"

	item, err := scanAdminUserRow(fakeScan(
		"user-1",
		"user@example.com",
		"18888888888",
		"OpenClaw 演示账号",
		true,
		&notes,
		now,
		now,
		int64(88800),
		int64(1200),
		int64(59400),
		int64(6),
		int64(3200),
		int64(2),
		int64(4),
		int64(12),
		int64(7),
	))
	if err != nil {
		t.Fatalf("scanAdminUserRow returned error: %v", err)
	}

	if item.User.Phone != "18888888888" {
		t.Fatalf("expected phone to be scanned, got %q", item.User.Phone)
	}
	if item.Notes == nil || *item.Notes != notes {
		t.Fatalf("expected notes to be preserved, got %#v", item.Notes)
	}
}

func TestScanAdminDeviceRowIncludesPlatformCapabilityColumns(t *testing.T) {
	now := time.Now().UTC()
	ownerID := "user-1"
	agentKey := "agent-key"
	localIP := "192.168.1.10"
	publicIP := "1.2.3.4"
	reasoningModel := "reasoning-model"
	chatModel := "chat-model"
	imageModel := "image-model"
	videoModel := "video-model"
	platformRevision := "rev-1"
	notes := "device notes"
	ownerEmail := "user@example.com"
	ownerName := "User Name"
	activationID := "activation-1"
	activationDeviceID := "device-1"
	activationOrderNo := "order-1"
	activationCode := "code-1"
	activationCodeHint := "尾号 001"
	activationStatus := "ready"
	activationNotes := "activation notes"

	item, err := scanAdminDeviceRow(fakeScan(
		"device-1",
		&ownerID,
		"DEVICE-001",
		&agentKey,
		"Factory Node",
		&localIP,
		&publicIP,
		&reasoningModel,
		&chatModel,
		&imageModel,
		&videoModel,
		[]byte(`[{"platformType":3,"slug":"douyin","label":"抖音","displayOrder":1,"visible":true,"loginEnabled":true,"publishEnabled":true}]`),
		&platformRevision,
		true,
		[]byte(`{"heartbeatIntervalSeconds":30}`),
		&now,
		&notes,
		now,
		now,
		int64(5),
		int64(4),
		int64(3),
		int64(2),
		int64(1),
		int64(0),
		int64(0),
		int64(0),
		int64(0),
		int64(0),
		int64(0),
		int64(0),
		int64(0),
		&ownerID,
		&ownerEmail,
		&ownerName,
		&activationID,
		&activationDeviceID,
		&activationOrderNo,
		&activationCode,
		&activationCodeHint,
		&activationStatus,
		&ownerID,
		&now,
		&activationNotes,
		&now,
		&now,
	))
	if err != nil {
		t.Fatalf("scanAdminDeviceRow returned error: %v", err)
	}

	if item.Device.PlatformCapabilitiesRevision == nil || *item.Device.PlatformCapabilitiesRevision != platformRevision {
		t.Fatalf("expected platform capability revision %q, got %#v", platformRevision, item.Device.PlatformCapabilitiesRevision)
	}
	if len(item.Device.PlatformCapabilities) != 1 || item.Device.PlatformCapabilities[0].Slug != "douyin" {
		t.Fatalf("expected platform capabilities to decode correctly, got %#v", item.Device.PlatformCapabilities)
	}
	if item.Owner == nil || item.Owner.Email != ownerEmail {
		t.Fatalf("expected owner summary to be populated, got %#v", item.Owner)
	}
	if item.Activation == nil || item.Activation.ID != activationID {
		t.Fatalf("expected activation config to be populated, got %#v", item.Activation)
	}
}

func fakeScan(values ...any) scanFn {
	return func(dest ...any) error {
		if len(dest) != len(values) {
			return &fakeScanError{expected: len(values), actual: len(dest)}
		}
		for index := range dest {
			target := reflect.ValueOf(dest[index])
			if target.Kind() != reflect.Ptr || target.IsNil() {
				panic("fakeScan received a non-pointer destination")
			}
			value := reflect.ValueOf(values[index])
			if !value.IsValid() {
				target.Elem().Set(reflect.Zero(target.Elem().Type()))
				continue
			}
			target.Elem().Set(value)
		}
		return nil
	}
}

type fakeScanError struct {
	expected int
	actual   int
}

func (e *fakeScanError) Error() string {
	return "fake scan destination count mismatch"
}
