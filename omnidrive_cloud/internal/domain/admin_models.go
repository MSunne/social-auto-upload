package domain

import (
	"encoding/json"
	"time"
)

type AdminIdentity struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"-"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	RoleIDs     []string   `json:"roleIds,omitempty"`
	Roles       []string   `json:"roles"`
	Permissions []string   `json:"permissions"`
	AuthMode    string     `json:"authMode"`
	IsActive    bool       `json:"isActive"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt,omitempty"`
}

type AdminSummary struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type AdminPermission struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

type AdminRole struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	IsSystem        bool      `json:"isSystem"`
	Permissions     []string  `json:"permissions"`
	PermissionCount int       `json:"permissionCount"`
	AdminCount      int64     `json:"adminCount"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type AdminPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type AdminUserSummary struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type AdminUserBillingSummary struct {
	CreditBalance            int64 `json:"creditBalance"`
	FrozenCreditBalance      int64 `json:"frozenCreditBalance"`
	TotalRechargeAmountCents int64 `json:"totalRechargeAmountCents"`
	TotalRechargeCount       int64 `json:"totalRechargeCount"`
	TotalConsumeCredits      int64 `json:"totalConsumeCredits"`
}

type AdminUserAssetSummary struct {
	DeviceCount       int64 `json:"deviceCount"`
	MediaAccountCount int64 `json:"mediaAccountCount"`
	PublishTaskCount  int64 `json:"publishTaskCount"`
	AIJobCount        int64 `json:"aiJobCount"`
}

type AdminUserActionState struct {
	CanUpdate     bool `json:"canUpdate"`
	CanDeactivate bool `json:"canDeactivate"`
	CanActivate   bool `json:"canActivate"`
}

type AdminUserRow struct {
	User    User                    `json:"user"`
	Billing AdminUserBillingSummary `json:"billing"`
	Assets  AdminUserAssetSummary   `json:"assets"`
	Notes   *string                 `json:"notes,omitempty"`
	Actions AdminUserActionState    `json:"actions"`
}

type AdminDeviceActionState struct {
	CanUpdate       bool `json:"canUpdate"`
	CanDisable      bool `json:"canDisable"`
	CanEnable       bool `json:"canEnable"`
	CanForceRelease bool `json:"canForceRelease"`
}

type AdminDeviceRow struct {
	Device  Device                 `json:"device"`
	Owner   *AdminUserSummary      `json:"owner,omitempty"`
	Actions AdminDeviceActionState `json:"actions"`
}

type AdminDeviceSummary struct {
	ID         string     `json:"id"`
	DeviceCode string     `json:"deviceCode"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	IsEnabled  bool       `json:"isEnabled"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}

type AdminAccountSummary struct {
	ID                  string     `json:"id"`
	Platform            string     `json:"platform"`
	AccountName         string     `json:"accountName"`
	Status              string     `json:"status"`
	LastMessage         *string    `json:"lastMessage,omitempty"`
	LastAuthenticatedAt *time.Time `json:"lastAuthenticatedAt,omitempty"`
}

type AdminSkillSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OutputType string `json:"outputType"`
	ModelName  string `json:"modelName"`
	IsEnabled  bool   `json:"isEnabled"`
}

type AdminAIModelSummary struct {
	ID        string `json:"id"`
	Vendor    string `json:"vendor"`
	ModelName string `json:"modelName"`
	Category  string `json:"category"`
	IsEnabled bool   `json:"isEnabled"`
}

type AdminMediaAccountRow struct {
	Account PlatformAccount              `json:"account"`
	Owner   *AdminUserSummary            `json:"owner,omitempty"`
	Device  AdminDeviceSummary           `json:"device"`
	Notes   *string                      `json:"notes,omitempty"`
	Actions AdminMediaAccountActionState `json:"actions"`
}

type AdminMediaAccountActionState struct {
	CanUpdate   bool `json:"canUpdate"`
	CanValidate bool `json:"canValidate"`
	CanDelete   bool `json:"canDelete"`
}

type AdminMediaAccountWorkspace struct {
	Record              AdminMediaAccountRow `json:"record"`
	RecentTasks         []PublishTask        `json:"recentTasks"`
	ActiveLoginSessions []LoginSession       `json:"activeLoginSessions"`
	RecentAudits        []AdminAuditRow      `json:"recentAudits"`
}

type AdminPublishTaskRow struct {
	Task               PublishTask            `json:"task"`
	Owner              *AdminUserSummary      `json:"owner,omitempty"`
	Device             AdminDeviceSummary     `json:"device"`
	Account            *AdminAccountSummary   `json:"account,omitempty"`
	Skill              *AdminSkillSummary     `json:"skill,omitempty"`
	Notes              *string                `json:"notes,omitempty"`
	ExceptionReason    *string                `json:"exceptionReason,omitempty"`
	RiskTags           []string               `json:"riskTags"`
	Readiness          PublishTaskReadiness   `json:"readiness"`
	BlockingDimensions []string               `json:"blockingDimensions,omitempty"`
	Bridge             PublishTaskBridgeState `json:"bridge"`
	Actions            PublishTaskActionState `json:"actions"`
	EventCount         int64                  `json:"eventCount"`
	ArtifactCount      int64                  `json:"artifactCount"`
	MaterialCount      int64                  `json:"materialCount"`
}

type AdminPublishTaskWorkspace struct {
	Record       AdminPublishTaskRow      `json:"record"`
	Events       []PublishTaskEvent       `json:"events"`
	Artifacts    []PublishTaskArtifact    `json:"artifacts"`
	Materials    []PublishTaskMaterialRef `json:"materials"`
	Runtime      *PublishTaskRuntimeState `json:"runtime,omitempty"`
	RecentAudits []AdminAuditRow          `json:"recentAudits"`
}

type AdminAIJobRow struct {
	Job                   AIJob                `json:"job"`
	Owner                 *AdminUserSummary    `json:"owner,omitempty"`
	Device                *AdminDeviceSummary  `json:"device,omitempty"`
	Skill                 *AdminSkillSummary   `json:"skill,omitempty"`
	Model                 *AdminAIModelSummary `json:"model,omitempty"`
	Notes                 *string              `json:"notes,omitempty"`
	ExceptionReason       *string              `json:"exceptionReason,omitempty"`
	RiskTags              []string             `json:"riskTags"`
	Bridge                AIJobBridgeState     `json:"bridge"`
	Actions               AIJobActionState     `json:"actions"`
	ArtifactCount         int64                `json:"artifactCount"`
	MirroredArtifactCount int64                `json:"mirroredArtifactCount"`
	PublishTaskCount      int64                `json:"publishTaskCount"`
}

type AdminAIJobWorkspace struct {
	Record             AdminAIJobRow       `json:"record"`
	Artifacts          []AIJobArtifact     `json:"artifacts"`
	PublishTasks       []PublishTask       `json:"publishTasks"`
	BillingUsageEvents []BillingUsageEvent `json:"billingUsageEvents"`
	RecentAudits       []AdminAuditRow     `json:"recentAudits"`
	ExecutionLogs      []AdminExecutionLog `json:"executionLogs"`
}

type AdminExecutionLog struct {
	ID        string          `json:"id"`
	Stage     string          `json:"stage"`
	Status    string          `json:"status"`
	Title     string          `json:"title"`
	Message   *string         `json:"message,omitempty"`
	Source    string          `json:"source"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type AdminUserWorkspace struct {
	Record         AdminUserRow      `json:"record"`
	BillingSummary BillingSummary    `json:"billingSummary"`
	Devices        []Device          `json:"devices"`
	MediaAccounts  []PlatformAccount `json:"mediaAccounts"`
	PublishTasks   []PublishTask     `json:"publishTasks"`
	AIJobs         []AIJob           `json:"aiJobs"`
	Orders         []RechargeOrder   `json:"orders"`
	WalletLedgers  []WalletLedger    `json:"walletLedgers"`
	RecentAudits   []AdminAuditRow   `json:"recentAudits"`
}

type AdminDeviceWorkspace struct {
	Record              AdminDeviceRow         `json:"record"`
	RecentTasks         []PublishTask          `json:"recentTasks"`
	RecentAIJobs        []AIJob                `json:"recentAiJobs"`
	ActiveLoginSessions []LoginSession         `json:"activeLoginSessions"`
	RecentAccounts      []PlatformAccount      `json:"recentAccounts"`
	MaterialRoots       []MaterialRoot         `json:"materialRoots"`
	SkillSyncStates     []DeviceSkillSyncState `json:"skillSyncStates"`
}

type AdminDeviceForceReleaseResult struct {
	Record                   AdminDeviceRow `json:"record"`
	ReleasedPublishTaskIDs   []string       `json:"releasedPublishTaskIds"`
	ReleasedAIJobIDs         []string       `json:"releasedAiJobIds"`
	ReleasedPublishTaskCount int64          `json:"releasedPublishTaskCount"`
	ReleasedAIJobCount       int64          `json:"releasedAiJobCount"`
	ServerTime               time.Time      `json:"serverTime"`
}

type AdminOrderRow struct {
	Order RechargeOrder    `json:"order"`
	User  AdminUserSummary `json:"user"`
}

type AdminWalletLedgerRow struct {
	Ledger WalletLedger     `json:"ledger"`
	User   AdminUserSummary `json:"user"`
}

type AdminBillingUsageEventRow struct {
	Event BillingUsageEvent `json:"event"`
	User  AdminUserSummary  `json:"user"`
}

type AdminOrderDetail struct {
	Record              AdminOrderRow        `json:"record"`
	Events              []RechargeOrderEvent `json:"events"`
	PaymentTransactions []PaymentTransaction `json:"paymentTransactions"`
	WalletLedgers       []WalletLedger       `json:"walletLedgers"`
}

type AdminWalletLedgerDetail struct {
	Record             AdminWalletLedgerRow     `json:"record"`
	Order              *AdminOrderRow           `json:"order,omitempty"`
	PaymentTransaction *PaymentTransaction      `json:"paymentTransaction,omitempty"`
	Adjustment         *WalletAdjustmentRequest `json:"adjustment,omitempty"`
}

type AdminWalletAdjustmentResult struct {
	Adjustment WalletAdjustmentRequest `json:"adjustment"`
	Ledger     AdminWalletLedgerRow    `json:"ledger"`
}

type AdminSupportRechargeRow struct {
	ID             string           `json:"id"`
	OrderNo        string           `json:"orderNo"`
	User           AdminUserSummary `json:"user"`
	RawStatus      string           `json:"rawStatus"`
	Status         string           `json:"status"`
	AmountCents    int64            `json:"amountCents"`
	BaseCredits    int64            `json:"baseCredits"`
	BonusCredits   int64            `json:"bonusCredits"`
	TotalCredits   int64            `json:"totalCredits"`
	SubmittedAt    time.Time        `json:"submittedAt"`
	ReviewedAt     *time.Time       `json:"reviewedAt,omitempty"`
	CreditedAt     *time.Time       `json:"creditedAt,omitempty"`
	ProviderStatus *string          `json:"providerStatus,omitempty"`
	Note           *string          `json:"note,omitempty"`
}

type AdminSupportRechargeSubmission struct {
	Status              string     `json:"status"`
	ContactChannel      *string    `json:"contactChannel,omitempty"`
	ContactHandle       *string    `json:"contactHandle,omitempty"`
	PaymentReference    *string    `json:"paymentReference,omitempty"`
	TransferAmountCents *int64     `json:"transferAmountCents,omitempty"`
	ProofURLs           []string   `json:"proofUrls"`
	CustomerNote        *string    `json:"customerNote,omitempty"`
	SubmittedAt         *time.Time `json:"submittedAt,omitempty"`
}

type AdminSupportRechargeReview struct {
	Status        string     `json:"status"`
	OperatorID    *string    `json:"operatorId,omitempty"`
	OperatorName  *string    `json:"operatorName,omitempty"`
	OperatorEmail *string    `json:"operatorEmail,omitempty"`
	Note          *string    `json:"note,omitempty"`
	ReviewedAt    *time.Time `json:"reviewedAt,omitempty"`
	CreditedAt    *time.Time `json:"creditedAt,omitempty"`
}

type AdminSupportRechargeActions struct {
	CanCredit bool `json:"canCredit"`
	CanReject bool `json:"canReject"`
}

type AdminSupportRechargeDetail struct {
	Record     AdminSupportRechargeRow        `json:"record"`
	Order      RechargeOrder                  `json:"order"`
	User       AdminUserSummary               `json:"user"`
	Submission AdminSupportRechargeSubmission `json:"submission"`
	Review     AdminSupportRechargeReview     `json:"review"`
	Events     []RechargeOrderEvent           `json:"events"`
	Actions    AdminSupportRechargeActions    `json:"actions"`
}

type AdminAuditRow struct {
	ID           string            `json:"id"`
	OwnerUserID  string            `json:"ownerUserId"`
	OwnerUser    *AdminUserSummary `json:"ownerUser,omitempty"`
	ActorType    string            `json:"actorType,omitempty"`
	Admin        *AdminSummary     `json:"admin,omitempty"`
	ResourceType string            `json:"resourceType"`
	ResourceID   *string           `json:"resourceId,omitempty"`
	Action       string            `json:"action"`
	Title        string            `json:"title"`
	Source       string            `json:"source"`
	Status       string            `json:"status"`
	Message      *string           `json:"message,omitempty"`
	Payload      []byte            `json:"payload,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
}

type AdminDashboardMetrics struct {
	UserCount              int64 `json:"userCount"`
	ActiveUserCount        int64 `json:"activeUserCount"`
	DeviceCount            int64 `json:"deviceCount"`
	OnlineDeviceCount      int64 `json:"onlineDeviceCount"`
	PublishTaskCount       int64 `json:"publishTaskCount"`
	FailedPublishTaskCount int64 `json:"failedPublishTaskCount"`
	AIJobCount             int64 `json:"aiJobCount"`
	FailedAIJobCount       int64 `json:"failedAiJobCount"`
}

type AdminDashboardFinance struct {
	OrderCount                  int64 `json:"orderCount"`
	PaidOrderCount              int64 `json:"paidOrderCount"`
	RechargeAmountCents         int64 `json:"rechargeAmountCents"`
	WalletLedgerCount           int64 `json:"walletLedgerCount"`
	PendingSupportRechargeCount int64 `json:"pendingSupportRechargeCount"`
	ManualSupportRechargeCount  int64 `json:"manualSupportRechargeCount"`
}

type AdminDashboardDistribution struct {
	PendingConsumeAmountCents    int64 `json:"pendingConsumeAmountCents"`
	PendingSettlementAmountCents int64 `json:"pendingSettlementAmountCents"`
	SettledAmountCents           int64 `json:"settledAmountCents"`
	PendingWithdrawalAmountCents int64 `json:"pendingWithdrawalAmountCents"`
}

type AdminDashboardQueues struct {
	NeedsVerifyTaskCount int64 `json:"needsVerifyTaskCount"`
	PendingAIJobCount    int64 `json:"pendingAiJobCount"`
	RunningAIJobCount    int64 `json:"runningAiJobCount"`
}

type AdminDashboardSummary struct {
	Metrics      AdminDashboardMetrics      `json:"metrics"`
	Finance      AdminDashboardFinance      `json:"finance"`
	Distribution AdminDashboardDistribution `json:"distribution"`
	Queues       AdminDashboardQueues       `json:"queues"`
	ServerTime   time.Time                  `json:"serverTime"`
}

type AdminOrderListSummary struct {
	TotalOrderCount           int64 `json:"totalOrderCount"`
	TotalAmountCents          int64 `json:"totalAmountCents"`
	TotalCreditAmount         int64 `json:"totalCreditAmount"`
	TotalBonusCreditAmount    int64 `json:"totalBonusCreditAmount"`
	PaidOrderCount            int64 `json:"paidOrderCount"`
	AwaitingManualReviewCount int64 `json:"awaitingManualReviewCount"`
	PendingPaymentCount       int64 `json:"pendingPaymentCount"`
	ProcessingCount           int64 `json:"processingCount"`
	RejectedCount             int64 `json:"rejectedCount"`
	ManualChannelCount        int64 `json:"manualChannelCount"`
}

type AdminWalletLedgerListSummary struct {
	TotalEntryCount int64 `json:"totalEntryCount"`
	TotalCreditIn   int64 `json:"totalCreditIn"`
	TotalCreditOut  int64 `json:"totalCreditOut"`
}

type AdminBillingUsageEventListSummary struct {
	TotalEventCount     int64 `json:"totalEventCount"`
	BilledCount         int64 `json:"billedCount"`
	FailedCount         int64 `json:"failedCount"`
	TotalDebitedCredits int64 `json:"totalDebitedCredits"`
}

type AdminSupportRechargeSummary struct {
	AwaitingSubmissionCount   int64 `json:"awaitingSubmissionCount"`
	PendingReviewCount        int64 `json:"pendingReviewCount"`
	RejectedCount             int64 `json:"rejectedCount"`
	CreditedCount             int64 `json:"creditedCount"`
	TotalRequestedAmountCents int64 `json:"totalRequestedAmountCents"`
	TotalBaseCredits          int64 `json:"totalBaseCredits"`
	TotalBonusCredits         int64 `json:"totalBonusCredits"`
}

type AdminDistributionRelationSummary struct {
	TotalCount    int64 `json:"totalCount"`
	ActiveCount   int64 `json:"activeCount"`
	InactiveCount int64 `json:"inactiveCount"`
}

type AdminCommissionListSummary struct {
	TotalCommissionAmountCents    int64 `json:"totalCommissionAmountCents"`
	PendingConsumeAmountCents     int64 `json:"pendingConsumeAmountCents"`
	PendingSettlementAmountCents  int64 `json:"pendingSettlementAmountCents"`
	SettledAmountCents            int64 `json:"settledAmountCents"`
	ReleasedButUnsettledAmountCts int64 `json:"releasedButUnsettledAmountCents"`
}

type AdminSettlementListSummary struct {
	TotalBatchCount      int64 `json:"totalBatchCount"`
	PendingBatchCount    int64 `json:"pendingBatchCount"`
	CompletedBatchCount  int64 `json:"completedBatchCount"`
	TotalAmountCents     int64 `json:"totalAmountCents"`
	PaidOutAmountCents   int64 `json:"paidOutAmountCents"`
	OutstandingAmountCts int64 `json:"outstandingAmountCents"`
}

type AdminWithdrawalListSummary struct {
	RequestedCount             int64 `json:"requestedCount"`
	ApprovedCount              int64 `json:"approvedCount"`
	RejectedCount              int64 `json:"rejectedCount"`
	PaidCount                  int64 `json:"paidCount"`
	PendingWithdrawalAmountCts int64 `json:"pendingWithdrawalAmountCents"`
	PaidWithdrawalAmountCents  int64 `json:"paidWithdrawalAmountCents"`
}

type AdminMediaAccountListSummary struct {
	TotalAccountCount    int64 `json:"totalAccountCount"`
	ActiveAccountCount   int64 `json:"activeAccountCount"`
	InactiveAccountCount int64 `json:"inactiveAccountCount"`
}

type AdminPublishTaskListSummary struct {
	TotalTaskCount       int64 `json:"totalTaskCount"`
	PendingCount         int64 `json:"pendingCount"`
	RunningCount         int64 `json:"runningCount"`
	NeedsVerifyCount     int64 `json:"needsVerifyCount"`
	CancelRequestedCount int64 `json:"cancelRequestedCount"`
	FailedCount          int64 `json:"failedCount"`
	CompletedCount       int64 `json:"completedCount"`
}

type AdminAIJobListSummary struct {
	TotalJobCount        int64 `json:"totalJobCount"`
	QueuedCount          int64 `json:"queuedCount"`
	RunningCount         int64 `json:"runningCount"`
	CompletedCount       int64 `json:"completedCount"`
	FailedCount          int64 `json:"failedCount"`
	CancelledCount       int64 `json:"cancelledCount"`
	PendingDeliveryCount int64 `json:"pendingDeliveryCount"`
}

type AdminPublishTaskBulkActionItem struct {
	RecordBefore  AdminPublishTaskRow  `json:"recordBefore"`
	RecordAfter   *AdminPublishTaskRow `json:"recordAfter,omitempty"`
	Status        string               `json:"status"`
	Message       *string              `json:"message,omitempty"`
	Action        string               `json:"action"`
	ArtifactCount int64                `json:"artifactCount,omitempty"`
}

type AdminPublishTaskBulkActionSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByAction       map[string]int64 `json:"byAction"`
}

type AdminPublishTaskBulkActionResult struct {
	Items      []AdminPublishTaskBulkActionItem  `json:"items"`
	Summary    AdminPublishTaskBulkActionSummary `json:"summary"`
	ServerTime time.Time                         `json:"serverTime"`
}

type AdminAIJobBulkActionItem struct {
	RecordBefore  AdminAIJobRow  `json:"recordBefore"`
	RecordAfter   *AdminAIJobRow `json:"recordAfter,omitempty"`
	Status        string         `json:"status"`
	Message       *string        `json:"message,omitempty"`
	Action        string         `json:"action"`
	ArtifactCount int64          `json:"artifactCount,omitempty"`
}

type AdminAIJobBulkActionSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByAction       map[string]int64 `json:"byAction"`
}

type AdminAIJobBulkActionResult struct {
	Items      []AdminAIJobBulkActionItem  `json:"items"`
	Summary    AdminAIJobBulkActionSummary `json:"summary"`
	ServerTime time.Time                   `json:"serverTime"`
}

type AdminUserBulkActionItem struct {
	RecordBefore AdminUserRow  `json:"recordBefore"`
	RecordAfter  *AdminUserRow `json:"recordAfter,omitempty"`
	Status       string        `json:"status"`
	Message      *string       `json:"message,omitempty"`
	Action       string        `json:"action"`
}

type AdminUserBulkActionSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByAction       map[string]int64 `json:"byAction"`
}

type AdminUserBulkActionResult struct {
	Items      []AdminUserBulkActionItem  `json:"items"`
	Summary    AdminUserBulkActionSummary `json:"summary"`
	ServerTime time.Time                  `json:"serverTime"`
}

type AdminDeviceBulkActionItem struct {
	RecordBefore             AdminDeviceRow  `json:"recordBefore"`
	RecordAfter              *AdminDeviceRow `json:"recordAfter,omitempty"`
	Status                   string          `json:"status"`
	Message                  *string         `json:"message,omitempty"`
	Action                   string          `json:"action"`
	ReleasedPublishTaskCount int64           `json:"releasedPublishTaskCount,omitempty"`
	ReleasedAIJobCount       int64           `json:"releasedAiJobCount,omitempty"`
}

type AdminDeviceBulkActionSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByAction       map[string]int64 `json:"byAction"`
}

type AdminDeviceBulkActionResult struct {
	Items      []AdminDeviceBulkActionItem  `json:"items"`
	Summary    AdminDeviceBulkActionSummary `json:"summary"`
	ServerTime time.Time                    `json:"serverTime"`
}

type AdminMediaAccountBulkActionItem struct {
	RecordBefore   AdminMediaAccountRow  `json:"recordBefore"`
	RecordAfter    *AdminMediaAccountRow `json:"recordAfter,omitempty"`
	Status         string                `json:"status"`
	Message        *string               `json:"message,omitempty"`
	Action         string                `json:"action"`
	LoginSessionID *string               `json:"loginSessionId,omitempty"`
	Deleted        bool                  `json:"deleted,omitempty"`
}

type AdminMediaAccountBulkActionSummary struct {
	SelectedCount  int64            `json:"selectedCount"`
	ProcessedCount int64            `json:"processedCount"`
	SuccessCount   int64            `json:"successCount"`
	SkippedCount   int64            `json:"skippedCount"`
	FailedCount    int64            `json:"failedCount"`
	ByStatus       map[string]int64 `json:"byStatus"`
	ByAction       map[string]int64 `json:"byAction"`
}

type AdminMediaAccountBulkActionResult struct {
	Items      []AdminMediaAccountBulkActionItem  `json:"items"`
	Summary    AdminMediaAccountBulkActionSummary `json:"summary"`
	ServerTime time.Time                          `json:"serverTime"`
}

type AdminDistributionRelationRow struct {
	ID        string           `json:"id"`
	Promoter  AdminUserSummary `json:"promoter"`
	Invitee   AdminUserSummary `json:"invitee"`
	Status    string           `json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	Notes     *string          `json:"notes,omitempty"`
}

type AdminCommissionRow struct {
	ID                        string           `json:"id"`
	Promoter                  AdminUserSummary `json:"promoter"`
	Invitee                   AdminUserSummary `json:"invitee"`
	Status                    string           `json:"status"`
	CommissionRate            float64          `json:"commissionRate"`
	CommissionBaseAmountCents int64            `json:"commissionBaseAmountCents"`
	AmountCents               int64            `json:"amountCents"`
	CreatedAt                 time.Time        `json:"createdAt"`
	ReleasedAt                *time.Time       `json:"releasedAt,omitempty"`
	SettledAt                 *time.Time       `json:"settledAt,omitempty"`
}

type AdminSettlementRow struct {
	ID               string     `json:"id"`
	BatchNo          string     `json:"batchNo"`
	Status           string     `json:"status"`
	ItemCount        int64      `json:"itemCount"`
	TotalAmountCents int64      `json:"totalAmountCents"`
	CreatedAt        time.Time  `json:"createdAt"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
	PaidAt           *time.Time `json:"paidAt,omitempty"`
	Reviewer         *string    `json:"reviewer,omitempty"`
	Operator         *string    `json:"operator,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
}

type AdminWithdrawalRow struct {
	ID            string           `json:"id"`
	Promoter      AdminUserSummary `json:"promoter"`
	Status        string           `json:"status"`
	AmountCents   int64            `json:"amountCents"`
	PayoutChannel *string          `json:"payoutChannel,omitempty"`
	AccountMasked *string          `json:"accountMasked,omitempty"`
	RequestedAt   time.Time        `json:"requestedAt"`
	ReviewedAt    *time.Time       `json:"reviewedAt,omitempty"`
	PaidAt        *time.Time       `json:"paidAt,omitempty"`
}

type AdminDistributionRule struct {
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Scope                    string            `json:"scope"`
	Status                   string            `json:"status"`
	CommissionRate           float64           `json:"commissionRate"`
	SettlementThresholdCents int64             `json:"settlementThresholdCents"`
	Promoter                 *AdminUserSummary `json:"promoter,omitempty"`
	Notes                    *string           `json:"notes,omitempty"`
	CreatedAt                time.Time         `json:"createdAt"`
	UpdatedAt                time.Time         `json:"updatedAt"`
}

type AdminWithdrawalDetail struct {
	Record               AdminWithdrawalRow `json:"record"`
	AvailableAmountCents int64              `json:"availableAmountCents"`
	Note                 *string            `json:"note,omitempty"`
	ProofURLs            []string           `json:"proofUrls"`
	PaymentReference     *string            `json:"paymentReference,omitempty"`
	Reviewer             *AdminSummary      `json:"reviewer,omitempty"`
	Operator             *AdminSummary      `json:"operator,omitempty"`
}

type AdminManualSupportConfig struct {
	Name      string `json:"name"`
	Contact   string `json:"contact"`
	QRCodeURL string `json:"qrCodeUrl"`
	Note      string `json:"note"`
}

type AdminSystemSettingsRecord struct {
	ID                        string                   `json:"id"`
	AIWorkerEnabled           bool                     `json:"aiWorkerEnabled"`
	PaymentChannels           []string                 `json:"paymentChannels"`
	BillingManualSupport      AdminManualSupportConfig `json:"billingManualSupport"`
	DefaultChatModel          string                   `json:"defaultChatModel"`
	DefaultImageModel         string                   `json:"defaultImageModel"`
	DefaultVideoModel         string                   `json:"defaultVideoModel"`
	StoryboardPrompt          string                   `json:"storyboardPrompt"`
	StoryboardModel           string                   `json:"storyboardModel"`
	StoryboardReferences      json.RawMessage          `json:"storyboardReferences,omitempty"`
	ImageStoryboardPrompt     string                   `json:"imageStoryboardPrompt"`
	ImageStoryboardModel      string                   `json:"imageStoryboardModel"`
	ImageStoryboardReferences json.RawMessage          `json:"imageStoryboardReferences,omitempty"`
	CreatedAt                 time.Time                `json:"createdAt"`
	UpdatedAt                 time.Time                `json:"updatedAt"`
}

type AdminSystemConfig struct {
	AuthMode                  string                   `json:"authMode"`
	AdminEmail                string                   `json:"adminEmail"`
	S3Configured              bool                     `json:"s3Configured"`
	S3Endpoint                string                   `json:"s3Endpoint"`
	S3Bucket                  string                   `json:"s3Bucket"`
	AIWorkerEnabled           bool                     `json:"aiWorkerEnabled"`
	PaymentChannels           []string                 `json:"paymentChannels"`
	BillingManualSupport      AdminManualSupportConfig `json:"billingManualSupport"`
	DefaultChatModel          string                   `json:"defaultChatModel"`
	DefaultImageModel         string                   `json:"defaultImageModel"`
	DefaultVideoModel         string                   `json:"defaultVideoModel"`
	StoryboardPrompt          string                   `json:"storyboardPrompt"`
	StoryboardModel           string                   `json:"storyboardModel"`
	StoryboardReferences      json.RawMessage          `json:"storyboardReferences,omitempty"`
	ImageStoryboardPrompt     string                   `json:"imageStoryboardPrompt"`
	ImageStoryboardModel      string                   `json:"imageStoryboardModel"`
	ImageStoryboardReferences json.RawMessage          `json:"imageStoryboardReferences,omitempty"`
	Notes                     []string                 `json:"notes"`
	UpdatedAt                 *time.Time               `json:"updatedAt,omitempty"`
}
