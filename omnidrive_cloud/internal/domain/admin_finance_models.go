package domain

type AdminPricingPackageSummary struct {
	TotalPackageCount int64 `json:"totalPackageCount"`
	EnabledCount      int64 `json:"enabledCount"`
	DisabledCount     int64 `json:"disabledCount"`
}
