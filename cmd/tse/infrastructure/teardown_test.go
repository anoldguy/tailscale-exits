package infrastructure

import (
	"testing"
)

func TestDetectLegacyResources(t *testing.T) {
	tests := []struct {
		name     string
		state    *InfrastructureState
		expected bool
	}{
		{
			name:     "No resources - not legacy",
			state:    &InfrastructureState{},
			expected: false,
		},
		{
			name: "All resources properly tagged - not legacy",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: map[string]string{"ManagedBy": "tse"},
				},
				IAMRole: &Resource{
					Name: "test-role",
					Tags: map[string]string{"ManagedBy": "tse"},
				},
				Lambda: &Resource{
					Name: "test-lambda",
					Tags: map[string]string{"ManagedBy": "tse"},
				},
			},
			expected: false,
		},
		{
			name: "All resources untagged - legacy detected",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: map[string]string{},
				},
				IAMRole: &Resource{
					Name: "test-role",
					Tags: map[string]string{},
				},
				Lambda: &Resource{
					Name: "test-lambda",
					Tags: map[string]string{},
				},
			},
			expected: true,
		},
		{
			name: "All resources with wrong tag value - legacy detected",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: map[string]string{"ManagedBy": "terraform"},
				},
				IAMRole: &Resource{
					Name: "test-role",
					Tags: map[string]string{"ManagedBy": "manual"},
				},
				Lambda: &Resource{
					Name: "test-lambda",
					Tags: map[string]string{"SomeOtherTag": "value"},
				},
			},
			expected: true,
		},
		{
			name: "One resource tagged, others not - treat as tse deployment",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: map[string]string{"ManagedBy": "tse"},
				},
				IAMRole: &Resource{
					Name: "test-role",
					Tags: map[string]string{},
				},
				Lambda: &Resource{
					Name: "test-lambda",
					Tags: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "Only Lambda tagged - treat as tse deployment",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: map[string]string{},
				},
				IAMRole: &Resource{
					Name: "test-role",
					Tags: map[string]string{},
				},
				Lambda: &Resource{
					Name: "test-lambda",
					Tags: map[string]string{"ManagedBy": "tse"},
				},
			},
			expected: false,
		},
		{
			name: "Partial deployment with tag - not legacy",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: map[string]string{"ManagedBy": "tse"},
				},
				// IAMRole and Lambda missing
			},
			expected: false,
		},
		{
			name: "Partial deployment without tag - legacy",
			state: &InfrastructureState{
				IAMRole: &Resource{
					Name: "test-role",
					Tags: map[string]string{"Project": "something-else"},
				},
				// LogGroup and Lambda missing, and no ManagedBy tag
			},
			expected: true,
		},
		{
			name: "Resources with nil Tags map - legacy",
			state: &InfrastructureState{
				LogGroup: &Resource{
					Name: "test-log",
					Tags: nil,
				},
				IAMRole: &Resource{
					Name: "test-role",
					Tags: nil,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLegacyResources(tt.state)
			if result != tt.expected {
				t.Errorf("detectLegacyResources() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
