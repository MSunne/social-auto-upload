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
	Load                  DeviceLoad      `json:"load"`
}

func (d Device) GetAgentKey() string {
	return d.AgentKey
}

type DeviceLoad struct {
	AccountCount                  int64 `json:"accountCount"`
	ActiveAccountCount            int64 `json:"activeAccountCount"`
	MaterialRootCount             int64 `json:"materialRootCount"`
	MaterialEntryCount            int64 `json:"materialEntryCount"`
	PendingTaskCount              int64 `json:"pendingTaskCount"`
	RunningTaskCount              int64 `json:"runningTaskCount"`
	NeedsVerifyTaskCount          int64 `json:"needsVerifyTaskCount"`
	CancelRequestedTaskCount      int64 `json:"cancelRequestedTaskCount"`
	FailedTaskCount               int64 `json:"failedTaskCount"`
	ActiveLoginSessionCount       int64 `json:"activeLoginSessionCount"`
	VerificationLoginSessionCount int64 `json:"verificationLoginSessionCount"`
}

type DeviceWorkspace struct {
	Device              Device            `json:"device"`
	RecentTasks         []PublishTask     `json:"recentTasks"`
	ActiveLoginSessions []LoginSession    `json:"activeLoginSessions"`
	RecentAccounts      []PlatformAccount `json:"recentAccounts"`
	MaterialRoots       []MaterialRoot    `json:"materialRoots"`
}

type PlatformAccount struct {
	ID                  string              `json:"id"`
	DeviceID            string              `json:"deviceId"`
	Platform            string              `json:"platform"`
	AccountName         string              `json:"accountName"`
	Status              string              `json:"status"`
	LastMessage         *string             `json:"lastMessage"`
	LastAuthenticatedAt *time.Time          `json:"lastAuthenticatedAt"`
	CreatedAt           time.Time           `json:"createdAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
	Load                PlatformAccountLoad `json:"load"`
}

type PlatformAccountLoad struct {
	TaskCount                     int64 `json:"taskCount"`
	PendingTaskCount              int64 `json:"pendingTaskCount"`
	RunningTaskCount              int64 `json:"runningTaskCount"`
	NeedsVerifyTaskCount          int64 `json:"needsVerifyTaskCount"`
	FailedTaskCount               int64 `json:"failedTaskCount"`
	ActiveLoginSessionCount       int64 `json:"activeLoginSessionCount"`
	VerificationLoginSessionCount int64 `json:"verificationLoginSessionCount"`
}

type PlatformAccountWorkspace struct {
	Account             PlatformAccount `json:"account"`
	RecentTasks         []PublishTask   `json:"recentTasks"`
	ActiveLoginSessions []LoginSession  `json:"activeLoginSessions"`
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
	ID               string           `json:"id"`
	OwnerUserID      string           `json:"ownerUserId"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	OutputType       string           `json:"outputType"`
	ModelName        string           `json:"modelName"`
	PromptTemplate   *string          `json:"promptTemplate"`
	ReferencePayload json.RawMessage  `json:"referencePayload,omitempty"`
	IsEnabled        bool             `json:"isEnabled"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	Load             ProductSkillLoad `json:"load"`
}

type ProductSkillLoad struct {
	AssetCount           int64 `json:"assetCount"`
	TaskCount            int64 `json:"taskCount"`
	PendingTaskCount     int64 `json:"pendingTaskCount"`
	RunningTaskCount     int64 `json:"runningTaskCount"`
	NeedsVerifyTaskCount int64 `json:"needsVerifyTaskCount"`
	FailedTaskCount      int64 `json:"failedTaskCount"`
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

type ProductSkillWorkspace struct {
	Skill       ProductSkill        `json:"skill"`
	Assets      []ProductSkillAsset `json:"assets"`
	RecentTasks []PublishTask       `json:"recentTasks"`
}

type MaterialRoot struct {
	ID           string    `json:"id"`
	DeviceID     string    `json:"deviceId"`
	RootName     string    `json:"rootName"`
	RootPath     string    `json:"rootPath"`
	IsAvailable  bool      `json:"isAvailable"`
	IsDirectory  bool      `json:"isDirectory"`
	LastSyncedAt time.Time `json:"lastSyncedAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type MaterialEntry struct {
	ID           string    `json:"id"`
	DeviceID     string    `json:"deviceId"`
	RootName     string    `json:"rootName"`
	RootPath     string    `json:"rootPath"`
	RelativePath string    `json:"relativePath"`
	ParentPath   string    `json:"parentPath"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	AbsolutePath *string   `json:"absolutePath"`
	SizeBytes    *int64    `json:"sizeBytes"`
	ModifiedAt   *string   `json:"modifiedAt"`
	Extension    *string   `json:"extension"`
	MimeType     *string   `json:"mimeType"`
	IsText       bool      `json:"isText"`
	PreviewText  *string   `json:"previewText"`
	IsAvailable  bool      `json:"isAvailable"`
	LastSyncedAt time.Time `json:"lastSyncedAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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
	LeaseOwnerDeviceID  *string         `json:"leaseOwnerDeviceId"`
	LeaseToken          *string         `json:"-"`
	LeaseExpiresAt      *time.Time      `json:"leaseExpiresAt"`
	AttemptCount        int             `json:"attemptCount"`
	CancelRequestedAt   *time.Time      `json:"cancelRequestedAt"`
	RunAt               *time.Time      `json:"runAt"`
	FinishedAt          *time.Time      `json:"finishedAt"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

type PublishTaskActionState struct {
	CanEdit   bool `json:"canEdit"`
	CanCancel bool `json:"canCancel"`
	CanRetry  bool `json:"canRetry"`
	CanDelete bool `json:"canDelete"`
}

type PublishTaskWorkspace struct {
	Task      PublishTask              `json:"task"`
	Device    *Device                  `json:"device,omitempty"`
	Account   *PlatformAccount         `json:"account,omitempty"`
	Skill     *ProductSkill            `json:"skill,omitempty"`
	Events    []PublishTaskEvent       `json:"events"`
	Artifacts []PublishTaskArtifact    `json:"artifacts"`
	Materials []PublishTaskMaterialRef `json:"materials"`
	Actions   PublishTaskActionState   `json:"actions"`
}

type PublishTaskEvent struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"taskId"`
	EventType string          `json:"eventType"`
	Source    string          `json:"source"`
	Status    string          `json:"status"`
	Message   *string         `json:"message"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

type PublishTaskArtifact struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"taskId"`
	ArtifactKey  string          `json:"artifactKey"`
	ArtifactType string          `json:"artifactType"`
	Source       string          `json:"source"`
	Title        *string         `json:"title"`
	FileName     *string         `json:"fileName"`
	MimeType     *string         `json:"mimeType"`
	StorageKey   *string         `json:"storageKey"`
	PublicURL    *string         `json:"publicUrl"`
	SizeBytes    *int64          `json:"sizeBytes"`
	TextContent  *string         `json:"textContent"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type PublishTaskMaterialRef struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"taskId"`
	DeviceID     string    `json:"deviceId"`
	RootName     string    `json:"rootName"`
	RelativePath string    `json:"relativePath"`
	Role         string    `json:"role"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	AbsolutePath *string   `json:"absolutePath"`
	SizeBytes    *int64    `json:"sizeBytes"`
	ModifiedAt   *string   `json:"modifiedAt"`
	Extension    *string   `json:"extension"`
	MimeType     *string   `json:"mimeType"`
	IsText       bool      `json:"isText"`
	PreviewText  *string   `json:"previewText"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AIModel struct {
	ID             string          `json:"id"`
	Vendor         string          `json:"vendor"`
	ModelName      string          `json:"modelName"`
	Category       string          `json:"category"`
	Description    *string         `json:"description"`
	PricingPayload json.RawMessage `json:"pricingPayload,omitempty"`
	IsEnabled      bool            `json:"isEnabled"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type AIJob struct {
	ID            string          `json:"id"`
	OwnerUserID   string          `json:"ownerUserId"`
	JobType       string          `json:"jobType"`
	ModelName     string          `json:"modelName"`
	Prompt        *string         `json:"prompt"`
	Status        string          `json:"status"`
	InputPayload  json.RawMessage `json:"inputPayload,omitempty"`
	OutputPayload json.RawMessage `json:"outputPayload,omitempty"`
	Message       *string         `json:"message"`
	CostCredits   int64           `json:"costCredits"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	FinishedAt    *time.Time      `json:"finishedAt"`
}

type BillingPackage struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Channel      string    `json:"channel"`
	PriceCents   int64     `json:"priceCents"`
	CreditAmount int64     `json:"creditAmount"`
	Badge        *string   `json:"badge"`
	Description  *string   `json:"description"`
	IsEnabled    bool      `json:"isEnabled"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type WalletLedger struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	EntryType     string    `json:"entryType"`
	AmountDelta   int64     `json:"amountDelta"`
	BalanceAfter  int64     `json:"balanceAfter"`
	Description   *string   `json:"description"`
	ReferenceType *string   `json:"referenceType"`
	ReferenceID   *string   `json:"referenceId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type HistoryItem struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Source     string     `json:"source"`
	Message    *string    `json:"message"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}
