package domain

import (
	"encoding/json"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Device struct {
	ID                    string          `json:"id"`
	OwnerUserID           *string         `json:"ownerUserId"`
	DeviceCode            string          `json:"deviceCode"`
	AgentKey              string          `json:"-"`
	Name                  string          `json:"name"`
	LocalIP               *string         `json:"localIp"`
	PublicIP              *string         `json:"publicIp"`
	DefaultReasoningModel *string         `json:"defaultReasoningModel"`
	IsEnabled             bool            `json:"isEnabled"`
	RuntimePayload        json.RawMessage `json:"runtimePayload,omitempty"`
	LastSeenAt            *time.Time      `json:"lastSeenAt"`
	Notes                 *string         `json:"notes"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
	Status                string          `json:"status"`
}

func (d Device) GetAgentKey() string {
	return d.AgentKey
}

type PlatformAccount struct {
	ID                  string     `json:"id"`
	DeviceID            string     `json:"deviceId"`
	Platform            string     `json:"platform"`
	AccountName         string     `json:"accountName"`
	Status              string     `json:"status"`
	LastMessage         *string    `json:"lastMessage"`
	LastAuthenticatedAt *time.Time `json:"lastAuthenticatedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type LoginSession struct {
	ID                  string          `json:"id"`
	DeviceID            string          `json:"deviceId"`
	UserID              string          `json:"userId"`
	Platform            string          `json:"platform"`
	AccountName         string          `json:"accountName"`
	Status              string          `json:"status"`
	QRData              *string         `json:"qrData"`
	VerificationPayload json.RawMessage `json:"verificationPayload,omitempty"`
	Message             *string         `json:"message"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

type LoginSessionAction struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionId"`
	ActionType string          `json:"actionType"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Status     string          `json:"status"`
	CreatedAt  time.Time       `json:"createdAt"`
	ConsumedAt *time.Time      `json:"consumedAt"`
}

type ProductSkill struct {
	ID               string          `json:"id"`
	OwnerUserID      string          `json:"ownerUserId"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	OutputType       string          `json:"outputType"`
	ModelName        string          `json:"modelName"`
	PromptTemplate   *string         `json:"promptTemplate"`
	ReferencePayload json.RawMessage `json:"referencePayload,omitempty"`
	IsEnabled        bool            `json:"isEnabled"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type ProductSkillAsset struct {
	ID          string    `json:"id"`
	SkillID     string    `json:"skillId"`
	OwnerUserID string    `json:"ownerUserId"`
	AssetType   string    `json:"assetType"`
	FileName    string    `json:"fileName"`
	MimeType    *string   `json:"mimeType"`
	StorageKey  *string   `json:"storageKey"`
	PublicURL   *string   `json:"publicUrl"`
	SizeBytes   *int64    `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type PublishTask struct {
	ID                  string          `json:"id"`
	DeviceID            string          `json:"deviceId"`
	AccountID           *string         `json:"accountId"`
	SkillID             *string         `json:"skillId"`
	Platform            string          `json:"platform"`
	AccountName         string          `json:"accountName"`
	Title               string          `json:"title"`
	ContentText         *string         `json:"contentText"`
	MediaPayload        json.RawMessage `json:"mediaPayload,omitempty"`
	Status              string          `json:"status"`
	Message             *string         `json:"message"`
	VerificationPayload json.RawMessage `json:"verificationPayload,omitempty"`
	RunAt               *time.Time      `json:"runAt"`
	FinishedAt          *time.Time      `json:"finishedAt"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}
