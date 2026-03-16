package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omnidrive_cloud/internal/domain"
)

const onlineWindow = 45 * time.Second
const publishTaskLeaseWindow = 90 * time.Second
const aiJobLeaseWindow = 90 * time.Second

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

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
	DefaultChatModel      *string
	DefaultImageModel     *string
	DefaultVideoModel     *string
	IsEnabled             *bool
}

type CreateRechargeOrderInput struct {
	ID                      string
	OrderNo                 string
	UserID                  string
	PackageID               *string
	PackageSnapshot         []byte
	Channel                 string
	Status                  string
	Subject                 string
	Body                    *string
	Currency                string
	AmountCents             int64
	CreditAmount            int64
	ManualBonusCreditAmount int64
	PaymentPayload          []byte
	CustomerServicePayload  []byte
	ProviderStatus          *string
	ExpiresAt               *time.Time
	TransactionID           string
	TransactionKind         string
	TransactionStatus       string
	TransactionOutTradeNo   string
	TransactionRequest      []byte
	TransactionResponse     []byte
}

type GrantWalletCreditsInput struct {
	UserID               string
	Amount               int64
	EntryType            *string
	Description          *string
	ReferenceType        *string
	ReferenceID          *string
	RechargeOrderID      *string
	PaymentTransactionID *string
	Metadata             []byte
}

type GrantQuotaInput struct {
	UserID        string
	MeterCode     string
	Amount        int64
	ExpiresAt     *time.Time
	SourceType    *string
	SourceID      *string
	Description   *string
	ReferenceType *string
	ReferenceID   *string
	Payload       []byte
}

type SubmitManualRechargeInput struct {
	Status                 string
	ProviderTransactionID  *string
	ProviderStatus         *string
	CustomerServicePayload []byte
	EventID                string
	EventType              string
	EventStatus            string
	EventMessage           *string
	EventPayload           []byte
}

type BillingPackageEntitlementInput struct {
	MeterCode   string
	GrantAmount int64
	GrantMode   string
	SortOrder   int
	Description *string
}

type CreateBillingPackageInput struct {
	ID                      string
	Name                    string
	PackageType             string
	PaymentChannels         []string
	Currency                string
	PriceCents              int64
	CreditAmount            int64
	ManualBonusCreditAmount int64
	Badge                   *string
	Description             *string
	PricingPayload          []byte
	ExpiresInDays           *int32
	IsEnabled               bool
	SortOrder               int
	Entitlements            []BillingPackageEntitlementInput
}

type UpdateBillingPackageInput struct {
	Name                    *string
	PackageType             *string
	PaymentChannels         []string
	PaymentChannelsTouched  bool
	Currency                *string
	PriceCents              *int64
	CreditAmount            *int64
	ManualBonusCreditAmount *int64
	Badge                   *string
	BadgeTouched            bool
	Description             *string
	DescriptionTouched      bool
	PricingPayload          []byte
	PricingPayloadTouched   bool
	ExpiresInDays           *int32
	ExpiresInDaysTouched    bool
	IsEnabled               *bool
	SortOrder               *int
	Entitlements            *[]BillingPackageEntitlementInput
}

type CreateWalletAdjustmentInput struct {
	UserID        string
	AmountDelta   int64
	Reason        string
	Note          *string
	EntryType     *string
	ReferenceType *string
	ReferenceID   *string
	AdminID       string
	AdminEmail    string
	AdminName     string
	Payload       []byte
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

type UpsertDeviceSkillSyncStateInput struct {
	DeviceID       string
	SkillID        string
	SyncStatus     string
	SyncedRevision *string
	AssetCount     int64
	Message        *string
	LastSyncedAt   *time.Time
}

type UpsertDeviceRetiredSkillAckInput struct {
	DeviceID           string
	SkillID            string
	Reason             string
	Message            *string
	LastAcknowledgedAt *time.Time
}

type SyncMaterialRootInput struct {
	DeviceID    string
	RootName    string
	RootPath    string
	IsAvailable bool
	IsDirectory bool
}

type SyncMaterialEntryInput struct {
	DeviceID     string
	RootName     string
	RootPath     string
	RelativePath string
	ParentPath   string
	Name         string
	Kind         string
	AbsolutePath *string
	SizeBytes    *int64
	ModifiedAt   *string
	Extension    *string
	MimeType     *string
	IsText       bool
	PreviewText  *string
	IsAvailable  bool
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
	ID            string
	DeviceID      string
	AccountID     *string
	SkillID       *string
	SkillRevision *string
	Platform      string
	AccountName   string
	Title         string
	ContentText   *string
	MediaPayload  []byte
	Status        string
	Message       *string
	RunAt         *time.Time
}

type ReplacePublishTaskMaterialRefInput struct {
	TaskID       string
	DeviceID     string
	RootName     string
	RelativePath string
	Role         string
	Name         string
	Kind         string
	AbsolutePath *string
	SizeBytes    *int64
	ModifiedAt   *string
	Extension    *string
	MimeType     *string
	IsText       bool
	PreviewText  *string
}

type ListPublishTasksFilter struct {
	DeviceID    string
	Status      string
	Platform    string
	AccountName string
	Limit       int
}

type SyncPublishTaskInput struct {
	ID                  string
	DeviceID            string
	AccountID           *string
	SkillID             *string
	SkillRevision       *string
	Platform            string
	AccountName         string
	Title               string
	ContentText         *string
	MediaPayload        []byte
	Status              string
	Message             *string
	VerificationPayload []byte
	LeaseToken          *string
	RunAt               *time.Time
	FinishedAt          *time.Time
}

type UpsertPublishTaskRuntimeStateInput struct {
	TaskID           string
	ExecutionPayload []byte
	ExecutionTouched bool
	LastAgentSyncAt  *time.Time
}

type UpdatePublishTaskInput struct {
	Title        *string
	ContentText  *string
	MediaPayload []byte
	MediaTouched bool
	Status       *string
	Message      *string
	RunAt        *time.Time
}

type CreatePublishTaskEventInput struct {
	ID        string
	TaskID    string
	EventType string
	Source    string
	Status    string
	Message   *string
	Payload   []byte
}

type UpsertPublishTaskArtifactInput struct {
	TaskID       string
	ArtifactKey  string
	ArtifactType string
	Source       string
	Title        *string
	FileName     *string
	MimeType     *string
	StorageKey   *string
	PublicURL    *string
	SizeBytes    *int64
	TextContent  *string
	Payload      []byte
}

type CreateAIJobInput struct {
	ID           string
	OwnerUserID  string
	DeviceID     *string
	SkillID      *string
	Source       string
	LocalTaskID  *string
	JobType      string
	ModelName    string
	Prompt       *string
	InputPayload []byte
	Status       string
	Message      *string
}

type UpdateAIJobInput struct {
	DeviceID                *string
	DeviceTouched           bool
	SkillID                 *string
	SkillTouched            bool
	LocalTaskID             *string
	LocalTaskTouched        bool
	Prompt                  *string
	Status                  *string
	InputPayload            []byte
	InputTouched            bool
	OutputPayload           []byte
	OutputTouched           bool
	Message                 *string
	CostCredits             *int64
	DeliveryStatus          *string
	DeliveryMessage         *string
	LocalPublishTaskID      *string
	LocalPublishTaskTouched bool
	FinishedAt              *time.Time
	FinishedTouched         bool
	DeliveredAt             *time.Time
	DeliveredTouched        bool
}

type ListAIJobsFilter struct {
	JobType  string
	Status   string
	SkillID  string
	DeviceID string
	Source   string
	Limit    int
}

type UpsertAIJobArtifactInput struct {
	JobID        string
	ArtifactKey  string
	ArtifactType string
	Source       string
	Title        *string
	FileName     *string
	MimeType     *string
	StorageKey   *string
	PublicURL    *string
	SizeBytes    *int64
	TextContent  *string
	DeviceID     *string
	RootName     *string
	RelativePath *string
	AbsolutePath *string
	Payload      []byte
}

type LinkAIJobPublishTaskInput struct {
	JobID       string
	TaskID      string
	OwnerUserID string
}

type CreateAuditEventInput struct {
	ID           string
	OwnerUserID  string
	ResourceType string
	ResourceID   *string
	Action       string
	Title        string
	Source       string
	Status       string
	Message      *string
	Payload      []byte
}

type ListHistoryFilter struct {
	Kind   string
	Status string
	Limit  int
}

type OverviewSummary struct {
	DeviceCount             int64                `json:"deviceCount"`
	OnlineDeviceCount       int64                `json:"onlineDeviceCount"`
	AccountCount            int64                `json:"accountCount"`
	MaterialRootCount       int64                `json:"materialRootCount"`
	MaterialEntryCount      int64                `json:"materialEntryCount"`
	SkillCount              int64                `json:"skillCount"`
	TaskCount               int64                `json:"taskCount"`
	PendingTaskCount        int64                `json:"pendingTaskCount"`
	RunningTaskCount        int64                `json:"runningTaskCount"`
	NeedsVerifyTaskCount    int64                `json:"needsVerifyTaskCount"`
	FailedTaskCount         int64                `json:"failedTaskCount"`
	ActiveLoginSessionCount int64                `json:"activeLoginSessionCount"`
	AIJobCount              int64                `json:"aiJobCount"`
	QueuedAIJobCount        int64                `json:"queuedAiJobCount"`
	RunningAIJobCount       int64                `json:"runningAiJobCount"`
	FailedAIJobCount        int64                `json:"failedAiJobCount"`
	BalanceCredits          int64                `json:"balanceCredits"`
	RecentTasks             []domain.PublishTask `json:"recentTasks"`
	RecentAIJobs            []domain.AIJob       `json:"recentAiJobs"`
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func PublishTaskLeaseTTL() time.Duration {
	return publishTaskLeaseWindow
}

func AIJobLeaseTTL() time.Duration {
	return aiJobLeaseWindow
}
