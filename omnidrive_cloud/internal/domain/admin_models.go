package domain

import "time"

type AdminIdentity struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	AuthMode    string   `json:"authMode"`
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

type AdminUserRow struct {
	User    User                    `json:"user"`
	Billing AdminUserBillingSummary `json:"billing"`
	Assets  AdminUserAssetSummary   `json:"assets"`
}

type AdminDeviceRow struct {
	Device Device            `json:"device"`
	Owner  *AdminUserSummary `json:"owner,omitempty"`
}

type AdminOrderRow struct {
	Order RechargeOrder    `json:"order"`
	User  AdminUserSummary `json:"user"`
}

type AdminWalletLedgerRow struct {
	Ledger WalletLedger     `json:"ledger"`
	User   AdminUserSummary `json:"user"`
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

type AdminSupportRechargeSummary struct {
	AwaitingSubmissionCount   int64 `json:"awaitingSubmissionCount"`
	PendingReviewCount        int64 `json:"pendingReviewCount"`
	RejectedCount             int64 `json:"rejectedCount"`
	CreditedCount             int64 `json:"creditedCount"`
	TotalRequestedAmountCents int64 `json:"totalRequestedAmountCents"`
	TotalBaseCredits          int64 `json:"totalBaseCredits"`
	TotalBonusCredits         int64 `json:"totalBonusCredits"`
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

type AdminSystemConfig struct {
	AuthMode        string   `json:"authMode"`
	AdminEmail      string   `json:"adminEmail"`
	S3Configured    bool     `json:"s3Configured"`
	S3Endpoint      string   `json:"s3Endpoint"`
	S3Bucket        string   `json:"s3Bucket"`
	AIWorkerEnabled bool     `json:"aiWorkerEnabled"`
	PaymentChannels []string `json:"paymentChannels"`
	Notes           []string `json:"notes"`
}
