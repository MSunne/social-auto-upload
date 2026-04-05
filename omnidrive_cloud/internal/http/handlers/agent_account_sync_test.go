package handlers

import "testing"

func TestNormalizeAgentAccountSyncStatus(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantStatus   string
		wantDelete   bool
		wantAccepted bool
	}{
		{
			name:         "active_status_is_accepted",
			input:        "active",
			wantStatus:   "active",
			wantAccepted: true,
		},
		{
			name:         "inactive_status_is_accepted",
			input:        "inactive",
			wantStatus:   "inactive",
			wantAccepted: true,
		},
		{
			name:         "deleted_status_requests_removal",
			input:        " deleted ",
			wantStatus:   "deleted",
			wantDelete:   true,
			wantAccepted: true,
		},
		{
			name:  "unknown_status_is_rejected",
			input: "needs_verify",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotDelete, gotAccepted := normalizeAgentAccountSyncStatus(tc.input)
			if gotStatus != tc.wantStatus || gotDelete != tc.wantDelete || gotAccepted != tc.wantAccepted {
				t.Fatalf(
					"normalizeAgentAccountSyncStatus(%q) = (%q, %v, %v), want (%q, %v, %v)",
					tc.input,
					gotStatus,
					gotDelete,
					gotAccepted,
					tc.wantStatus,
					tc.wantDelete,
					tc.wantAccepted,
				)
			}
		})
	}
}
