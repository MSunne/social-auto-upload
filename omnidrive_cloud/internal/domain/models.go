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
	Device              Device                 `json:"device"`
	RecentTasks         []PublishTask          `json:"recentTasks"`
	ActiveLoginSessions []LoginSession         `json:"activeLoginSessions"`
	RecentAccounts      []PlatformAccount      `json:"recentAccounts"`
	MaterialRoots       []MaterialRoot         `json:"materialRoots"`
	SkillSyncStates     []DeviceSkillSyncState `json:"skillSyncStates"`
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
	AIJobCount           int64 `json:"aiJobCount"`
	ActiveAIJobCount     int64 `json:"activeAiJobCount"`
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

type DeviceSkillSyncState struct {
	ID              string     `json:"id"`
	DeviceID        string     `json:"deviceId"`
	SkillID         string     `json:"skillId"`
	SyncStatus      string     `json:"syncStatus"`
	SyncedRevision  *string    `json:"syncedRevision"`
	DesiredRevision *string    `json:"desiredRevision,omitempty"`
	IsCurrent       bool       `json:"isCurrent"`
	NeedsSync       bool       `json:"needsSync"`
	AssetCount      int64      `json:"assetCount"`
	Message         *string    `json:"message"`
	LastSyncedAt    *time.Time `json:"lastSyncedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type DeviceRetiredSkillAck struct {
	ID                 string    `json:"id"`
	DeviceID           string    `json:"deviceId"`
	SkillID            string    `json:"skillId"`
	Reason             string    `json:"reason"`
	Message            *string   `json:"message,omitempty"`
	LastAcknowledgedAt time.Time `json:"lastAcknowledgedAt"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type ProductSkillWorkspace struct {
	Skill        ProductSkill           `json:"skill"`
	Assets       []ProductSkillAsset    `json:"assets"`
	RecentTasks  []PublishTask          `json:"recentTasks"`
	RecentAIJobs []AIJob                `json:"recentAiJobs"`
	DeviceSyncs  []DeviceSkillSyncState `json:"deviceSyncs"`
}

type ProductSkillImpactWorkspace struct {
	Skill      ProductSkill                 `json:"skill"`
	Items      []PublishTaskDiagnosticItem  `json:"items"`
	Summary    PublishTaskDiagnosticSummary `json:"summary"`
	ServerTime time.Time                    `json:"serverTime"`
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

type MaterialImpactSummary struct {
	TaskCount    int64            `json:"taskCount"`
	ReadyCount   int64            `json:"readyCount"`
	BlockedCount int64            `json:"blockedCount"`
	ByStatus     map[string]int64 `json:"byStatus"`
	ByDimension  map[string]int64 `json:"byDimension"`
	ByIssueCode  map[string]int64 `json:"byIssueCode"`
}

type MaterialEntryWorkspace struct {
	DeviceID         string                      `json:"deviceId"`
	Root             MaterialRoot                `json:"root"`
	Entry            MaterialEntry               `json:"entry"`
	Scope            string                      `json:"scope"`
	ReferencingTasks []PublishTaskDiagnosticItem `json:"referencingTasks"`
	Summary          MaterialImpactSummary       `json:"summary"`
}

type PublishTask struct {
	ID                  string          `json:"id"`
	DeviceID            string          `json:"deviceId"`
	AccountID           *string         `json:"accountId"`
	SkillID             *string         `json:"skillId"`
	SkillRevision       *string         `json:"skillRevision"`
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
	CanEdit             bool `json:"canEdit"`
	CanCancel           bool `json:"canCancel"`
	CanRetry            bool `json:"canRetry"`
	CanDelete           bool `json:"canDelete"`
	CanForceRelease     bool `json:"canForceRelease"`
	CanResume           bool `json:"canResume"`
	CanResolveManual    bool `json:"canResolveManual"`
	CanRefreshMaterials bool `json:"canRefreshMaterials"`
	CanRefreshSkill     bool `json:"canRefreshSkill"`
}

type PublishTaskReadiness struct {
	DeviceReady            bool     `json:"deviceReady"`
	AccountReady           bool     `json:"accountReady"`
	SkillReady             bool     `json:"skillReady"`
	SkillRevisionMatched   bool     `json:"skillRevisionMatched"`
	SkillSyncedToDevice    bool     `json:"skillSyncedToDevice"`
	MaterialsReady         bool     `json:"materialsReady"`
	TotalMaterialCount     int64    `json:"totalMaterialCount"`
	AvailableMaterialCount int64    `json:"availableMaterialCount"`
	MissingMaterialCount   int64    `json:"missingMaterialCount"`
	DriftedMaterialCount   int64    `json:"driftedMaterialCount"`
	IssueCodes             []string `json:"issueCodes"`
	Issues                 []string `json:"issues"`
}

type PublishTaskRuntimeState struct {
	TaskID           string          `json:"taskId"`
	ExecutionPayload json.RawMessage `json:"executionPayload,omitempty"`
	LastAgentSyncAt  *time.Time      `json:"lastAgentSyncAt"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type PublishTaskBridgeState struct {
	Origin          string     `json:"origin"`
	LocalSource     *string    `json:"localSource,omitempty"`
	Stage           *string    `json:"stage,omitempty"`
	LocalStatus     *string    `json:"localStatus,omitempty"`
	WorkerName      *string    `json:"workerName,omitempty"`
	UpdatedAt       *string    `json:"updatedAt,omitempty"`
	StartedAt       *string    `json:"startedAt,omitempty"`
	FinishedAt      *string    `json:"finishedAt,omitempty"`
	LastAgentSyncAt *time.Time `json:"lastAgentSyncAt,omitempty"`
	HasActiveLease  bool       `json:"hasActiveLease"`
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
	Readiness PublishTaskReadiness     `json:"readiness"`
	Runtime   *PublishTaskRuntimeState `json:"runtime,omitempty"`
	Bridge    PublishTaskBridgeState   `json:"bridge"`
}

type PublishTaskDiagnosticItem struct {
	Task               PublishTask          `json:"task"`
	Readiness          PublishTaskReadiness `json:"readiness"`
	BlockingDimensions []string             `json:"blockingDimensions"`
}

type PublishTaskDiagnosticSummary struct {
	TotalCount   int64            `json:"totalCount"`
	ReadyCount   int64            `json:"readyCount"`
	BlockedCount int64            `json:"blockedCount"`
	ByStatus     map[string]int64 `json:"byStatus"`
	ByDimension  map[string]int64 `json:"byDimension"`
	ByIssueCode  map[string]int64 `json:"byIssueCode"`
}

type AgentSkillPackage struct {
	Revision string                `json:"revision"`
	Skill    ProductSkill          `json:"skill"`
	Assets   []ProductSkillAsset   `json:"assets"`
	Sync     *DeviceSkillSyncState `json:"sync,omitempty"`
}

type AgentRetiredSkillItem struct {
	SkillID        string     `json:"skillId"`
	Reason         string     `json:"reason"`
	Name           *string    `json:"name,omitempty"`
	OutputType     *string    `json:"outputType,omitempty"`
	Message        *string    `json:"message,omitempty"`
	SyncedRevision *string    `json:"syncedRevision,omitempty"`
	LastSyncedAt   *time.Time `json:"lastSyncedAt,omitempty"`
	LastChangedAt  time.Time  `json:"lastChangedAt"`
}

type AgentSkillManifestSummary struct {
	ActiveCount   int64 `json:"activeCount"`
	RetiredCount  int64 `json:"retiredCount"`
	DisabledCount int64 `json:"disabledCount"`
	DeletedCount  int64 `json:"deletedCount"`
}

type AgentPublishTaskPackage struct {
	Task        PublishTask              `json:"task"`
	Account     *PlatformAccount         `json:"account,omitempty"`
	Skill       *ProductSkill            `json:"skill,omitempty"`
	SkillAssets []ProductSkillAsset      `json:"skillAssets"`
	Materials   []PublishTaskMaterialRef `json:"materials"`
	Readiness   PublishTaskReadiness     `json:"readiness"`
	Runtime     *PublishTaskRuntimeState `json:"runtime,omitempty"`
}

type AgentPublishTaskQueueItem struct {
	Task               PublishTask          `json:"task"`
	Readiness          PublishTaskReadiness `json:"readiness"`
	BlockingDimensions []string             `json:"blockingDimensions"`
}

type AgentPublishTaskQueueSummary struct {
	ReadyCount   int64            `json:"readyCount"`
	BlockedCount int64            `json:"blockedCount"`
	ByStatus     map[string]int64 `json:"byStatus"`
	ByDimension  map[string]int64 `json:"byDimension"`
	ByIssueCode  map[string]int64 `json:"byIssueCode"`
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

type PublishTaskMaterialRefreshIssue struct {
	Code         string                  `json:"code"`
	Message      string                  `json:"message"`
	RootName     string                  `json:"rootName"`
	RelativePath string                  `json:"relativePath"`
	Role         string                  `json:"role"`
	PreviousRef  *PublishTaskMaterialRef `json:"previousRef,omitempty"`
	CurrentEntry *MaterialEntry          `json:"currentEntry,omitempty"`
}

type PublishTaskMaterialRefreshResult struct {
	Task           PublishTask                       `json:"task"`
	Materials      []PublishTaskMaterialRef          `json:"materials"`
	Readiness      PublishTaskReadiness              `json:"readiness"`
	RefreshedCount int64                             `json:"refreshedCount"`
	ChangedCount   int64                             `json:"changedCount"`
	MissingCount   int64                             `json:"missingCount"`
	Issues         []PublishTaskMaterialRefreshIssue `json:"issues"`
}

type PublishTaskSkillRefreshResult struct {
	Task             PublishTask          `json:"task"`
	Skill            *ProductSkill        `json:"skill,omitempty"`
	Readiness        PublishTaskReadiness `json:"readiness"`
	PreviousRevision *string              `json:"previousRevision,omitempty"`
	CurrentRevision  *string              `json:"currentRevision,omitempty"`
	RevisionChanged  bool                 `json:"revisionChanged"`
}

type PublishTaskBulkRepairItem struct {
	Task              PublishTask                       `json:"task"`
	Status            string                            `json:"status"`
	Message           *string                           `json:"message,omitempty"`
	AppliedOperations []string                          `json:"appliedOperations"`
	ReadinessBefore   PublishTaskReadiness              `json:"readinessBefore"`
	ReadinessAfter    PublishTaskReadiness              `json:"readinessAfter"`
	MaterialRefresh   *PublishTaskMaterialRefreshResult `json:"materialRefresh,omitempty"`
	SkillRefresh      *PublishTaskSkillRefreshResult    `json:"skillRefresh,omitempty"`
}

type PublishTaskBulkRepairSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByOperation    map[string]int64 `json:"byOperation"`
}

type PublishTaskBulkRepairResult struct {
	Items      []PublishTaskBulkRepairItem  `json:"items"`
	Summary    PublishTaskBulkRepairSummary `json:"summary"`
	ServerTime time.Time                    `json:"serverTime"`
}

type PublishTaskBulkActionItem struct {
	TaskBefore    PublishTask  `json:"taskBefore"`
	TaskAfter     *PublishTask `json:"taskAfter,omitempty"`
	Status        string       `json:"status"`
	Message       *string      `json:"message,omitempty"`
	Action        string       `json:"action"`
	ArtifactCount int64        `json:"artifactCount,omitempty"`
}

type PublishTaskBulkActionSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByAction       map[string]int64 `json:"byAction"`
}

type PublishTaskBulkActionResult struct {
	Items      []PublishTaskBulkActionItem  `json:"items"`
	Summary    PublishTaskBulkActionSummary `json:"summary"`
	ServerTime time.Time                    `json:"serverTime"`
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
	ID                 string          `json:"id"`
	OwnerUserID        string          `json:"ownerUserId"`
	DeviceID           *string         `json:"deviceId"`
	SkillID            *string         `json:"skillId"`
	Source             string          `json:"source"`
	LocalTaskID        *string         `json:"localTaskId"`
	JobType            string          `json:"jobType"`
	ModelName          string          `json:"modelName"`
	Prompt             *string         `json:"prompt"`
	Status             string          `json:"status"`
	InputPayload       json.RawMessage `json:"inputPayload,omitempty"`
	OutputPayload      json.RawMessage `json:"outputPayload,omitempty"`
	Message            *string         `json:"message"`
	CostCredits        int64           `json:"costCredits"`
	LeaseOwnerDeviceID *string         `json:"leaseOwnerDeviceId"`
	LeaseToken         *string         `json:"leaseToken"`
	LeaseExpiresAt     *time.Time      `json:"leaseExpiresAt"`
	DeliveryStatus     string          `json:"deliveryStatus"`
	DeliveryMessage    *string         `json:"deliveryMessage"`
	LocalPublishTaskID *string         `json:"localPublishTaskId"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	DeliveredAt        *time.Time      `json:"deliveredAt"`
	FinishedAt         *time.Time      `json:"finishedAt"`
}

type AIJobActionState struct {
	CanEdit              bool `json:"canEdit"`
	CanCancel            bool `json:"canCancel"`
	CanRetry             bool `json:"canRetry"`
	CanCreatePublishTask bool `json:"canCreatePublishTask"`
	CanForceRelease      bool `json:"canForceRelease"`
}

type AIJobArtifact struct {
	ID           string          `json:"id"`
	JobID        string          `json:"jobId"`
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
	DeviceID     *string         `json:"deviceId"`
	RootName     *string         `json:"rootName"`
	RelativePath *string         `json:"relativePath"`
	AbsolutePath *string         `json:"absolutePath"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type AIJobBridgeState struct {
	Source                 string  `json:"source"`
	GenerationSide         string  `json:"generationSide"`
	TargetDeviceID         *string `json:"targetDeviceId"`
	LocalTaskID            *string `json:"localTaskId"`
	LocalPublishTaskID     *string `json:"localPublishTaskId"`
	DeliveryStage          string  `json:"deliveryStage"`
	ArtifactCount          int     `json:"artifactCount"`
	MirroredArtifactCount  int     `json:"mirroredArtifactCount"`
	LinkedPublishTaskCount int     `json:"linkedPublishTaskCount"`
}

type AIJobWorkspace struct {
	Job          AIJob            `json:"job"`
	Model        *AIModel         `json:"model,omitempty"`
	Skill        *ProductSkill    `json:"skill,omitempty"`
	Artifacts    []AIJobArtifact  `json:"artifacts"`
	PublishTasks []PublishTask    `json:"publishTasks"`
	Bridge       AIJobBridgeState `json:"bridge"`
	Actions      AIJobActionState `json:"actions"`
}

type AgentAIJobPackage struct {
	Job         AIJob               `json:"job"`
	Skill       *ProductSkill       `json:"skill,omitempty"`
	SkillAssets []ProductSkillAsset `json:"skillAssets"`
	Artifacts   []AIJobArtifact     `json:"artifacts"`
}

type AgentAIJobDeliveryItem struct {
	Job       AIJob            `json:"job"`
	Artifacts []AIJobArtifact  `json:"artifacts"`
	Bridge    AIJobBridgeState `json:"bridge"`
	Actions   AIJobActionState `json:"actions"`
}

type BillingPackage struct {
	ID              string                      `json:"id"`
	Name            string                      `json:"name"`
	PackageType     string                      `json:"packageType"`
	Channel         string                      `json:"channel"`
	PaymentChannels []string                    `json:"paymentChannels"`
	Currency        string                      `json:"currency"`
	PriceCents      int64                       `json:"priceCents"`
	CreditAmount    int64                       `json:"creditAmount"`
	Badge           *string                     `json:"badge"`
	Description     *string                     `json:"description"`
	PricingPayload  json.RawMessage             `json:"pricingPayload,omitempty"`
	ExpiresInDays   *int32                      `json:"expiresInDays,omitempty"`
	IsEnabled       bool                        `json:"isEnabled"`
	SortOrder       int                         `json:"sortOrder"`
	Entitlements    []BillingPackageEntitlement `json:"entitlements"`
	CreatedAt       time.Time                   `json:"createdAt"`
	UpdatedAt       time.Time                   `json:"updatedAt"`
}

type BillingPackageEntitlement struct {
	ID          string    `json:"id"`
	PackageID   string    `json:"packageId"`
	MeterCode   string    `json:"meterCode"`
	MeterName   *string   `json:"meterName,omitempty"`
	Unit        *string   `json:"unit,omitempty"`
	GrantAmount int64     `json:"grantAmount"`
	GrantMode   string    `json:"grantMode"`
	SortOrder   int       `json:"sortOrder"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type BillingPricingRule struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	MeterCode         string    `json:"meterCode"`
	MeterName         *string   `json:"meterName,omitempty"`
	AppliesTo         string    `json:"appliesTo"`
	ModelName         *string   `json:"modelName,omitempty"`
	JobType           *string   `json:"jobType,omitempty"`
	ChargeMode        string    `json:"chargeMode"`
	QuotaMeterCode    *string   `json:"quotaMeterCode,omitempty"`
	QuotaMeterName    *string   `json:"quotaMeterName,omitempty"`
	UnitSize          int64     `json:"unitSize"`
	WalletDebitAmount int64     `json:"walletDebitAmount"`
	SortOrder         int       `json:"sortOrder"`
	Description       *string   `json:"description,omitempty"`
	IsEnabled         bool      `json:"isEnabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type BillingQuotaBalance struct {
	MeterCode        string     `json:"meterCode"`
	MeterName        string     `json:"meterName"`
	Unit             string     `json:"unit"`
	RemainingTotal   int64      `json:"remainingTotal"`
	NearestExpiresAt *time.Time `json:"nearestExpiresAt,omitempty"`
}

type BillingSummary struct {
	CreditBalance        int64                 `json:"creditBalance"`
	FrozenCreditBalance  int64                 `json:"frozenCreditBalance"`
	PendingRechargeCount int64                 `json:"pendingRechargeCount"`
	QuotaBalances        []BillingQuotaBalance `json:"quotaBalances"`
}

type WalletLedger struct {
	ID                   string          `json:"id"`
	UserID               string          `json:"userId"`
	EntryType            string          `json:"entryType"`
	AmountDelta          int64           `json:"amountDelta"`
	BalanceBefore        int64           `json:"balanceBefore"`
	BalanceAfter         int64           `json:"balanceAfter"`
	MeterCode            *string         `json:"meterCode,omitempty"`
	Quantity             *int64          `json:"quantity,omitempty"`
	Unit                 *string         `json:"unit,omitempty"`
	UnitPriceCredits     *int64          `json:"unitPriceCredits,omitempty"`
	Description          *string         `json:"description"`
	ReferenceType        *string         `json:"referenceType"`
	ReferenceID          *string         `json:"referenceId"`
	RechargeOrderID      *string         `json:"rechargeOrderId,omitempty"`
	PaymentTransactionID *string         `json:"paymentTransactionId,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
}

type RechargeOrder struct {
	ID                     string          `json:"id"`
	OrderNo                string          `json:"orderNo"`
	UserID                 string          `json:"userId"`
	PackageID              *string         `json:"packageId,omitempty"`
	PackageSnapshot        json.RawMessage `json:"packageSnapshot,omitempty"`
	Channel                string          `json:"channel"`
	Status                 string          `json:"status"`
	Subject                string          `json:"subject"`
	Body                   *string         `json:"body,omitempty"`
	Currency               string          `json:"currency"`
	AmountCents            int64           `json:"amountCents"`
	CreditAmount           int64           `json:"creditAmount"`
	PaymentPayload         json.RawMessage `json:"paymentPayload,omitempty"`
	CustomerServicePayload json.RawMessage `json:"customerServicePayload,omitempty"`
	ProviderTransactionID  *string         `json:"providerTransactionId,omitempty"`
	ProviderStatus         *string         `json:"providerStatus,omitempty"`
	ExpiresAt              *time.Time      `json:"expiresAt,omitempty"`
	PaidAt                 *time.Time      `json:"paidAt,omitempty"`
	ClosedAt               *time.Time      `json:"closedAt,omitempty"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
}

type RechargeOrderEvent struct {
	ID              string          `json:"id"`
	RechargeOrderID string          `json:"rechargeOrderId"`
	UserID          string          `json:"userId"`
	EventType       string          `json:"eventType"`
	Status          string          `json:"status"`
	Message         *string         `json:"message,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
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
