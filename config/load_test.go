package config

import "testing"

func TestValidateRepoPath(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{
			name:      "valid single segment",
			value:     "charts",
			wantError: false,
		},
		{
			name:      "valid multi segment",
			value:     "org/charts",
			wantError: false,
		},
		{
			name:      "empty string",
			value:     "",
			wantError: true,
		},
		{
			name:      "whitespace only",
			value:     "   ",
			wantError: true,
		},
		{
			name:      "whitespace around valid value",
			value:     "  charts  ",
			wantError: false,
		},
		{
			name:      "starts with slash",
			value:     "/charts",
			wantError: true,
		},
		{
			name:      "ends with slash",
			value:     "charts/",
			wantError: true,
		},
		{
			name:      "starts and ends with slash",
			value:     "/charts/",
			wantError: true,
		},
		{
			name:      "single slash only",
			value:     "/",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoPath("TestRepo", tt.value)

			if tt.wantError && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
