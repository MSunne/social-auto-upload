package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omnidrive_cloud/internal/domain"
)

const onlineWindow = 45 * time.Second

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func computeDeviceStatus(lastSeenAt *time.Time) string {
	if lastSeenAt == nil {
		return "offline"
	}
	if time.Since(lastSeenAt.UTC()) <= onlineWindow {
		return "online"
	}
	return "offline"
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func bytesOrNil(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}

type UserWithPassword struct {
	User         domain.User
	PasswordHash string
}

type CreateUserInput struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
}

type UpdateDeviceInput struct {
	Name                  *string
	DefaultReasoningModel *string
	IsEnabled             *bool
}

type HeartbeatInput struct {
	DeviceCode     string
	AgentKey       string
	DeviceName     string
	LocalIP        *string
	PublicIP       *string
	RuntimePayload []byte
}

type CreateLoginSessionInput struct {
	ID          string
	DeviceID    string
	UserID      string
	Platform    string
	AccountName string
	Status      string
	Message     *string
}

type LoginEventInput struct {
	Status              string
	Message             *string
	QRData              *string
	VerificationPayload []byte
}

type CreateLoginActionInput struct {
	ID         string
	SessionID  string
	ActionType string
	Payload    []byte
}

type CreateSkillInput struct {
	ID               string
	OwnerUserID      string
	Name             string
	Description      string
	OutputType       string
	ModelName        string
	PromptTemplate   *string
	ReferencePayload []byte
	IsEnabled        bool
}

type CreateSkillAssetInput struct {
	ID          string
	SkillID     string
	OwnerUserID string
	AssetType   string
	FileName    string
	MimeType    *string
	StorageKey  *string
	PublicURL   *string
	SizeBytes   *int64
}

type UpdateSkillInput struct {
	Name             *string
	Description      *string
	OutputType       *string
	ModelName        *string
	PromptTemplate   *string
	ReferencePayload []byte
	ReferenceTouched bool
	IsEnabled        *bool
}

type CreatePublishTaskInput struct {
	ID           string
	DeviceID     string
	AccountID    *string
	SkillID      *string
	Platform     string
	AccountName  string
	Title        string
	ContentText  *string
	MediaPayload []byte
	Status       string
	Message      *string
	RunAt        *time.Time
}

type SyncPublishTaskInput struct {
	ID                  string
	DeviceID            string
	AccountID           *string
	SkillID             *string
	Platform            string
	AccountName         string
	Title               string
	ContentText         *string
	MediaPayload        []byte
	Status              string
	Message             *string
	VerificationPayload []byte
	RunAt               *time.Time
	FinishedAt          *time.Time
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
