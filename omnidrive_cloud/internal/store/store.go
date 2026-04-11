package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omnidrive_cloud/internal/domain"
)

const onlineWindow = 2 * time.Minute
const minOnlineWindow = 90 * time.Second
const maxOnlineWindow = 5 * time.Minute
const publishTaskLeaseWindow = 90 * time.Second
const aiJobLeaseWindow = 90 * time.Second

type Store struct {
	pool *pgxpool.Pool
}

// 创建存储层相关实例，组装运行所需依赖并返回给上层流程复用。
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type deviceRuntimeHeartbeatHints struct {
	HeartbeatIntervalSeconds int    `json:"heartbeatIntervalSeconds"`
	HeartbeatInterval        int    `json:"heartbeatInterval"`
	BridgeStatus             string `json:"bridgeStatus"`
	BridgeLastError          string `json:"bridgeLastError"`
	LastError                string `json:"lastError"`
	CloudReachable           *bool  `json:"cloudReachable"`
}

// 根据最后心跳时间和运行时载荷计算设备状态，供列表查询和诊断逻辑复用。
func computeDeviceStatus(lastSeenAt *time.Time, runtimePayload []byte) string {
	if lastSeenAt == nil {
		return "offline"
	}
	if time.Since(lastSeenAt.UTC()) <= onlineWindowForRuntimePayload(runtimePayload) {
		return "online"
	}
	return "offline"
}

// 根据最后心跳时间和运行时载荷计算设备桥接状态，供在线诊断和监控逻辑复用。
func computeDeviceBridgeStatus(lastSeenAt *time.Time, runtimePayload []byte) string {
	if computeDeviceStatus(lastSeenAt, runtimePayload) != "online" {
		return "offline"
	}
	if len(runtimePayload) == 0 {
		return "unknown"
	}

	var hints deviceRuntimeHeartbeatHints
	if err := json.Unmarshal(runtimePayload, &hints); err != nil {
		return "unknown"
	}

	switch strings.ToLower(strings.TrimSpace(hints.BridgeStatus)) {
	case "healthy":
		return "healthy"
	case "degraded":
		return "degraded"
	case "unknown":
		return "unknown"
	}

	if hints.CloudReachable != nil && !*hints.CloudReachable {
		return "degraded"
	}
	if strings.TrimSpace(hints.BridgeLastError) != "" || strings.TrimSpace(hints.LastError) != "" {
		return "degraded"
	}
	return "unknown"
}

// 根据运行时载荷推导设备在线判定窗口，避免不同心跳频率下出现误判。
func onlineWindowForRuntimePayload(runtimePayload []byte) time.Duration {
	if len(runtimePayload) == 0 {
		return onlineWindow
	}

	var hints deviceRuntimeHeartbeatHints
	if err := json.Unmarshal(runtimePayload, &hints); err != nil {
		return onlineWindow
	}

	intervalSeconds := hints.HeartbeatIntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = hints.HeartbeatInterval
	}
	if intervalSeconds <= 0 {
		return onlineWindow
	}

	window := time.Duration(intervalSeconds) * time.Second * 4
	if window < minOnlineWindow {
		return minOnlineWindow
	}
	if window > maxOnlineWindow {
		return maxOnlineWindow
	}
	return window
}

// 构建设备在线 SQL 条件，供查询语句在数据库层复用统一判定逻辑。
func deviceOnlineSQLPredicate(tableAlias string) string {
	qualified := strings.TrimSpace(tableAlias)
	if qualified == "" {
		qualified = "devices"
	}
	intervalValue := fmt.Sprintf(
		"COALESCE(%[1]s.runtime_payload->>'heartbeatIntervalSeconds', %[1]s.runtime_payload->>'heartbeatInterval', '')",
		qualified,
	)
	return fmt.Sprintf(
		`(%[1]s.last_seen_at IS NOT NULL AND %[1]s.last_seen_at >= NOW() - make_interval(secs => LEAST(%[2]d, GREATEST(%[3]d, CASE WHEN %[4]s ~ '^[0-9]+$' AND (%[4]s)::INT > 0 THEN (%[4]s)::INT * 4 ELSE %[5]d END))))`,
		qualified,
		int(maxOnlineWindow/time.Second),
		int(minOnlineWindow/time.Second),
		intervalValue,
		int(onlineWindow/time.Second),
	)
}

// 构建设备健康在线 SQL 条件，供查询语句在数据库层复用统一判定逻辑。
func deviceHealthyOnlineSQLPredicate(tableAlias string) string {
	qualified := strings.TrimSpace(tableAlias)
	if qualified == "" {
		qualified = "devices"
	}
	return fmt.Sprintf("(%s AND LOWER(COALESCE(%s.runtime_payload->>'bridgeStatus', 'unknown')) = 'healthy')", deviceOnlineSQLPredicate(qualified), qualified)
}

// 将非空字符串转换为指针，统一存储层对可选字符串字段的入参表达。
func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// 解包可选字符串指针，统一存储层对空值字段的回写行为。
func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// 将非零时间转换为 UTC 指针，统一数据库写入时的时间表达。
func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

// 在字节切片为空时返回 nil，统一 JSON 和二进制字段的持久化语义。
func bytesOrNil(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}

// 将任意结构序列化为 JSON 字节，失败时直接 panic 以暴露调用方数据错误。
func mustJSONBytes(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}

type UserWithPassword struct {
	User         domain.User
	PasswordHash string
}

type CreateUserInput struct {
	ID           string
	Email        string
	Phone        string
	Name         string
	PasswordHash string
}

type CreateUserRegistrationInput struct {
	CreateUserInput
	PartnerCode string
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
	UserID                       string
	Amount                       int64
	EntryType                    *string
	Description                  *string
	ReferenceType                *string
	ReferenceID                  *string
	RechargeOrderID              *string
	PaymentTransactionID         *string
	DistributionCommissionItemID *string
	ReleaseUnitCredits           int64
	Metadata                     []byte
}

type GrantQuotaInput struct {
	UserID                       string
	MeterCode                    string
	Amount                       int64
	ExpiresAt                    *time.Time
	SourceType                   *string
	SourceID                     *string
	RechargeOrderID              *string
	DistributionCommissionItemID *string
	ReleaseUnitCredits           int64
	Description                  *string
	ReferenceType                *string
	ReferenceID                  *string
	Payload                      []byte
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
	DeviceCode        string
	AgentKey          string
	DeviceName        string
	DeviceFingerprint string
	LocalIP           *string
	PublicIP          *string
	RuntimePayload    []byte
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
	ID                       string
	OwnerUserID              string
	DeviceID                 *string
	Name                     string
	Description              string
	OutputType               string
	ModelName                string
	FixedDurationSeconds     *int
	PromptTemplate           *string
	StoryboardPromptTemplate *string
	PublishPromptTemplate    *string
	PublishIntroEnabled      bool
	CoverPromptTemplate      *string
	Topics                   []string
	ReferencePayload         []byte
	ExecutionTime            *time.Time
	RepeatDaily              bool
	StoryboardEnabled        bool
	NextRunAt                *time.Time
	IsEnabled                bool
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
	Name                     *string
	Description              *string
	OutputType               *string
	ModelName                *string
	FixedDurationSeconds     *int
	FixedDurationTouched     bool
	PromptTemplate           *string
	StoryboardPromptTemplate *string
	PublishPromptTemplate    *string
	PublishIntroEnabled      *bool
	CoverPromptTemplate      *string
	Topics                   []string
	TopicsTouched            bool
	ReferencePayload         []byte
	ReferenceTouched         bool
	DeviceID                 *string
	DeviceTouched            bool
	ExecutionTime            *time.Time
	ExecutionTouched         bool
	RepeatDaily              *bool
	StoryboardEnabled        *bool
	NextRunAt                *time.Time
	NextRunTouched           bool
	LastRunAt                *time.Time
	LastRunTouched           bool
	IsEnabled                *bool
}

type CreateWorkflowDurationRuleInput struct {
	ID                  string
	WorkflowCode        string
	OutputType          string
	DurationSeconds     int
	SegmentSeconds      int
	SpecialPriceCredits *int64
	IsEnabled           bool
	SortOrder           int
	Description         *string
}

type UpdateWorkflowDurationRuleInput struct {
	WorkflowCode               *string
	OutputType                 *string
	DurationSeconds            *int
	SegmentSeconds             *int
	SpecialPriceCredits        *int64
	SpecialPriceCreditsTouched bool
	IsEnabled                  *bool
	SortOrder                  *int
	Description                *string
	DescriptionTouched         bool
}

type CreateAIBillingSessionInput struct {
	ID                  string
	UserID              string
	SourceType          string
	SourceID            string
	WorkflowCode        string
	OutputType          string
	DurationSeconds     int
	SegmentSeconds      int
	SpecialRuleID       *string
	SpecialPriceCredits *int64
	PlannedCredits      int64
	BilledCredits       int64
	RefundedCredits     int64
	Status              string
	Message             *string
	Payload             []byte
}

type CreateAIBillingItemInput struct {
	ID               string
	SessionID        string
	UserID           string
	SourceType       string
	SourceID         string
	ItemKey          string
	ItemType         string
	Label            string
	ModelName        *string
	ModelAlias       *string
	MeterCode        *string
	Quantity         int64
	Unit             string
	UnitPriceCredits int64
	PlannedCredits   int64
	BilledCredits    int64
	RefundedCredits  int64
	SortOrder        int
	IsVisible        bool
	Status           string
	WalletLedgerID   *string
	Payload          []byte
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
	AccountID   string
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
	RunAt        *time.Time
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
	RunAt                   *time.Time
	RunAtTouched            bool
	FinishedAt              *time.Time
	FinishedTouched         bool
	DeliveredAt             *time.Time
	DeliveredTouched        bool
}

type ListAIJobsFilter struct {
	JobType       string
	Status        string
	SkillID       string
	DeviceID      string
	AccountID     string
	Source        string
	ExcludeSource string
	PayloadMode   string
	Limit         int
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

type CreateDigitalHumanTaskInput struct {
	ID                       string
	OwnerUserID              string
	AIJobID                  *string
	Mode                     string
	Source                   string
	Status                   string
	ModelName                string
	CharacterAsset           []byte
	GoodsAsset               []byte
	RefAudioAsset            []byte
	GoodsTitle               *string
	GoodsText                string
	EstimatedDurationSeconds int
	EstimatedCreditsMillis   int64
	BillingStatus            string
	BillingPayload           []byte
	Progress                 []byte
	RequestPayload           []byte
}

type UpdateDigitalHumanTaskExecutionInput struct {
	Status                *string
	RemoteTaskID          *string
	RemoteTaskTouched     bool
	ResultAsset           []byte
	ResultAssetTouched    bool
	ActualDurationSeconds *int
	ActualDurationTouched bool
	FinalCreditsMillis    *int64
	FinalCreditsTouched   bool
	BillingStatus         *string
	BillingStatusTouched  bool
	BillingPayload        []byte
	BillingPayloadTouched bool
	Progress              []byte
	ProgressTouched       bool
	RemoteResponsePayload []byte
	RemotePayloadTouched  bool
	ErrorMessage          *string
	StartedAt             *time.Time
	StartedTouched        bool
	CompletedAt           *time.Time
	CompletedTouched      bool
	WorkingDir            *string
	WorkingDirTouched     bool
}

type ListDigitalHumanTasksFilter struct {
	AIJobID string
	Mode    string
	Status  string
	Limit   int
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

// 执行数据库连通性检查，供服务启动和健康探针确认连接池是否可用。
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// 处理发布任务租约TTL相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func PublishTaskLeaseTTL() time.Duration {
	return publishTaskLeaseWindow
}

// 处理AI作业租约TTL相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func AIJobLeaseTTL() time.Duration {
	return aiJobLeaseWindow
}
