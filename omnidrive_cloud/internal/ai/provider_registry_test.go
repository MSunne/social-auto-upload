package ai

import (
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestProviderSourceNormalizesVendorAliases(t *testing.T) {
	tests := []struct {
		name  string
		model *domain.AIModel
		want  string
	}{
		{
			name:  "lconai chinese alias",
			model: &domain.AIModel{Vendor: "万象龙坤"},
			want:  "lconai",
		},
		{
			name:  "apiyi chinese alias",
			model: &domain.AIModel{Vendor: "万象引擎"},
			want:  "apiyi",
		},
		{
			name:  "apiyi vendor inferred from base url",
			model: &domain.AIModel{Vendor: "阿里", BaseURL: testStringPtr("https://api.apiyi.com/v1/chat/completions")},
			want:  "apiyi",
		},
		{
			name:  "lconai vendor inferred from base url",
			model: &domain.AIModel{Vendor: "", BaseURL: testStringPtr("https://n.lconai.com/v1/videos")},
			want:  "lconai",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerSource(tc.model); got != tc.want {
				t.Fatalf("providerSource returned %q, want %q", got, tc.want)
			}
		})
	}
}

func testStringPtr(value string) *string {
	return &value
}
