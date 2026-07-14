package masker

import (
	"strings"
	"testing"
)

func TestMasker(t *testing.T) {
	masker := NewMasker()

	tests := []struct {
		name           string
		input          string
		expectedSub    string // substring expected in output
		expectedNotSub string // substring not expected in output
		minMasked      int
	}{
		{
			name:           "AWS Key",
			input:          "My aws key is AKIA1234567890ABCDEF, please keep it safe.",
			expectedSub:    "[AWS_KEY]",
			expectedNotSub: "AKIA1234567890ABCDEF",
			minMasked:      1,
		},
		{
			name:           "Email and IP",
			input:          "Contact support@example.com or connect to 192.168.1.100.",
			expectedSub:    "[EMAIL]",
			expectedNotSub: "support@example.com",
			minMasked:      2, // both email and IP
		},
		{
			name:           "Slack Webhook",
			input:          "Post message to https://hooks.slack.com/services/T12345/B67890/abc123xyz",
			expectedSub:    "[SLACK_WEBHOOK]",
			expectedNotSub: "hooks.slack.com/services",
			minMasked:      1,
		},
		{
			name:           "Database URI Password",
			input:          "postgres://admin:superSecretPass123@localhost:5432/mydb",
			expectedSub:    "[MASKED_PASSWORD]",
			expectedNotSub: "superSecretPass123",
			minMasked:      1,
		},
		{
			name:           "Generic Secret assignment",
			input:          `const apiKey = "d3b07384d113edec49eaa6238ad5ff00";`,
			expectedSub:    `[MASKED_SECRET]`,
			expectedNotSub: "d3b07384d113edec49eaa6238ad5ff00",
			minMasked:      1,
		},
		{
			name:           "High Entropy stand-alone",
			input:          `Please check this key: 4a7d90e2b3c40f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a`,
			expectedSub:    `[MASKED_HIGH_ENTROPY_KEY]`,
			expectedNotSub: "4a7d90e2b3c40f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a",
			minMasked:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, count := masker.Mask(tt.input)

			if !strings.Contains(output, tt.expectedSub) {
				t.Errorf("expected output to contain %q, but got: %q", tt.expectedSub, output)
			}
			if strings.Contains(output, tt.expectedNotSub) {
				t.Errorf("expected output to NOT contain %q, but got: %q", tt.expectedNotSub, output)
			}
			if count < tt.minMasked {
				t.Errorf("expected at least %d items masked, got %d", tt.minMasked, count)
			}
		})
	}
}
