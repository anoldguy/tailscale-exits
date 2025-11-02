package infrastructure

import (
	"testing"
)

func TestState_AllResourcesPresent(t *testing.T) {
	state := &InfrastructureState{
		LogGroup:    &Resource{Name: "test-log"},
		IAMRole:     &Resource{Name: "test-role"},
		Lambda:      &Resource{Name: "test-lambda"},
		FunctionURL: "https://test.lambda-url.us-east-2.on.aws/",
	}
	state.Policies.Managed = true
	state.Policies.InlineName = "test-policy"

	if !state.Exists() {
		t.Error("Expected Exists()=true when resources present")
	}
	if !state.IsComplete() {
		t.Error("Expected IsComplete()=true when all resources present")
	}
	missing := state.Missing()
	if len(missing) != 0 {
		t.Errorf("Expected no missing resources, got %v", missing)
	}
}

func TestState_NoResources(t *testing.T) {
	state := &InfrastructureState{}

	if state.Exists() {
		t.Error("Expected Exists()=false when no resources present")
	}
	if state.IsComplete() {
		t.Error("Expected IsComplete()=false when no resources present")
	}

	missing := state.Missing()
	if len(missing) != 6 {
		t.Errorf("Expected 6 missing resources, got %d: %v", len(missing), missing)
	}

	// Check all expected resources are listed as missing
	expectedMissing := map[string]bool{
		"CloudWatch Log Group":      true,
		"IAM Role":                  true,
		"Managed Policy Attachment": true,
		"Inline Policy":             true,
		"Lambda Function":           true,
		"Function URL":              true,
	}

	for _, resource := range missing {
		if !expectedMissing[resource] {
			t.Errorf("Unexpected missing resource: %s", resource)
		}
	}
}

func TestState_PartialDeployment(t *testing.T) {
	tests := []struct {
		name           string
		state          *InfrastructureState
		expectExists   bool
		expectComplete bool
		expectMissing  []string
	}{
		{
			name: "Only log group",
			state: &InfrastructureState{
				LogGroup: &Resource{Name: "test-log"},
			},
			expectExists:   true,
			expectComplete: false,
			expectMissing: []string{
				"IAM Role",
				"Managed Policy Attachment",
				"Inline Policy",
				"Lambda Function",
				"Function URL",
			},
		},
		{
			name: "IAM role without policies",
			state: &InfrastructureState{
				IAMRole: &Resource{Name: "test-role"},
			},
			expectExists:   true,
			expectComplete: false,
			expectMissing: []string{
				"CloudWatch Log Group",
				"Managed Policy Attachment",
				"Inline Policy",
				"Lambda Function",
				"Function URL",
			},
		},
		{
			name: "Lambda without URL",
			state: func() *InfrastructureState {
				s := &InfrastructureState{
					LogGroup: &Resource{Name: "test-log"},
					IAMRole:  &Resource{Name: "test-role"},
					Lambda:   &Resource{Name: "test-lambda"},
				}
				s.Policies.Managed = true
				s.Policies.InlineName = "test-policy"
				return s
			}(),
			expectExists:   true,
			expectComplete: false,
			expectMissing:  []string{"Function URL"},
		},
		{
			name: "Role with managed policy but no inline policy",
			state: func() *InfrastructureState {
				s := &InfrastructureState{
					LogGroup:    &Resource{Name: "test-log"},
					IAMRole:     &Resource{Name: "test-role"},
					Lambda:      &Resource{Name: "test-lambda"},
					FunctionURL: "https://test.lambda-url.us-east-2.on.aws/",
				}
				s.Policies.Managed = true
				return s
			}(),
			expectExists:   true,
			expectComplete: false,
			expectMissing:  []string{"Inline Policy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExists := tt.state.Exists()
			gotComplete := tt.state.IsComplete()
			gotMissing := tt.state.Missing()

			if gotExists != tt.expectExists {
				t.Errorf("Expected Exists()=%v, got %v", tt.expectExists, gotExists)
			}
			if gotComplete != tt.expectComplete {
				t.Errorf("Expected IsComplete()=%v, got %v", tt.expectComplete, gotComplete)
			}
			if len(gotMissing) != len(tt.expectMissing) {
				t.Errorf("Expected %d missing resources, got %d: %v",
					len(tt.expectMissing), len(gotMissing), gotMissing)
			}

			// Verify specific missing resources
			missing := make(map[string]bool)
			for _, r := range gotMissing {
				missing[r] = true
			}
			for _, expected := range tt.expectMissing {
				if !missing[expected] {
					t.Errorf("Expected %q to be in missing resources, but it wasn't", expected)
				}
			}
		})
	}
}

func TestState_Idempotent(t *testing.T) {
	state := &InfrastructureState{
		LogGroup: &Resource{Name: "test-log"},
		IAMRole:  &Resource{Name: "test-role"},
	}
	state.Policies.InlineName = "test-policy"

	// Call methods multiple times - results should be identical
	firstExists := state.Exists()
	firstComplete := state.IsComplete()
	firstMissing := state.Missing()

	secondExists := state.Exists()
	secondComplete := state.IsComplete()
	secondMissing := state.Missing()

	// Results should be identical
	if firstExists != secondExists {
		t.Errorf("Exists() changed from %v to %v", firstExists, secondExists)
	}
	if firstComplete != secondComplete {
		t.Errorf("IsComplete() changed from %v to %v", firstComplete, secondComplete)
	}
	if len(firstMissing) != len(secondMissing) {
		t.Errorf("Missing() length changed from %d to %d", len(firstMissing), len(secondMissing))
	}

	// Check that contents match
	for i, m := range firstMissing {
		if i >= len(secondMissing) || m != secondMissing[i] {
			t.Errorf("Missing resources differ at index %d: %v vs %v", i, firstMissing, secondMissing)
			break
		}
	}
}
