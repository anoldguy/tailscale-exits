package tailscale

import (
	"testing"
)

func TestEnsureTagOwner(t *testing.T) {
	tests := []struct {
		name     string
		policy   *ACLPolicy
		tag      string
		owner    string
		want     bool // whether changes were made
		validate func(*testing.T, *ACLPolicy)
	}{
		{
			name:   "add tag to empty policy",
			policy: &ACLPolicy{},
			tag:    "tag:exitnode",
			owner:  "autogroup:admin",
			want:   true,
			validate: func(t *testing.T, p *ACLPolicy) {
				if p.TagOwners == nil {
					t.Error("TagOwners should be initialized")
				}
				if owners, ok := p.TagOwners["tag:exitnode"]; !ok {
					t.Error("tag:exitnode should exist")
				} else if len(owners) != 1 || owners[0] != "autogroup:admin" {
					t.Errorf("expected [autogroup:admin], got %v", owners)
				}
			},
		},
		{
			name: "tag already exists with same owner",
			policy: &ACLPolicy{
				TagOwners: map[string][]string{
					"tag:exitnode": {"autogroup:admin"},
				},
			},
			tag:   "tag:exitnode",
			owner: "autogroup:admin",
			want:  false, // no changes
			validate: func(t *testing.T, p *ACLPolicy) {
				if owners := p.TagOwners["tag:exitnode"]; len(owners) != 1 {
					t.Errorf("should still have one owner, got %v", owners)
				}
			},
		},
		{
			name: "tag exists with different owner",
			policy: &ACLPolicy{
				TagOwners: map[string][]string{
					"tag:exitnode": {"user@example.com"},
				},
			},
			tag:   "tag:exitnode",
			owner: "autogroup:admin",
			want:  false, // don't modify existing ownership
			validate: func(t *testing.T, p *ACLPolicy) {
				if owners := p.TagOwners["tag:exitnode"]; len(owners) != 1 || owners[0] != "user@example.com" {
					t.Errorf("should preserve original owner, got %v", owners)
				}
			},
		},
		{
			name:   "nil policy",
			policy: nil,
			tag:    "tag:exitnode",
			owner:  "autogroup:admin",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureTagOwner(tt.policy, tt.tag, tt.owner)
			if got != tt.want {
				t.Errorf("EnsureTagOwner() = %v, want %v", got, tt.want)
			}
			if tt.validate != nil && tt.policy != nil {
				tt.validate(t, tt.policy)
			}
		})
	}
}

func TestEnsureAutoApprover(t *testing.T) {
	tests := []struct {
		name     string
		policy   *ACLPolicy
		tag      string
		want     bool
		validate func(*testing.T, *ACLPolicy)
	}{
		{
			name:   "add to empty policy",
			policy: &ACLPolicy{},
			tag:    "tag:exitnode",
			want:   true,
			validate: func(t *testing.T, p *ACLPolicy) {
				if p.AutoApprovers == nil {
					t.Error("AutoApprovers should be initialized")
				}
				if len(p.AutoApprovers.ExitNode) != 1 || p.AutoApprovers.ExitNode[0] != "tag:exitnode" {
					t.Errorf("expected [tag:exitnode], got %v", p.AutoApprovers.ExitNode)
				}
			},
		},
		{
			name: "tag already exists",
			policy: &ACLPolicy{
				AutoApprovers: &AutoApprovers{
					ExitNode: []string{"tag:exitnode"},
				},
			},
			tag:  "tag:exitnode",
			want: false,
			validate: func(t *testing.T, p *ACLPolicy) {
				if len(p.AutoApprovers.ExitNode) != 1 {
					t.Errorf("should still have one tag, got %v", p.AutoApprovers.ExitNode)
				}
			},
		},
		{
			name: "add to existing approvers",
			policy: &ACLPolicy{
				AutoApprovers: &AutoApprovers{
					ExitNode: []string{"tag:other"},
				},
			},
			tag:  "tag:exitnode",
			want: true,
			validate: func(t *testing.T, p *ACLPolicy) {
				if len(p.AutoApprovers.ExitNode) != 2 {
					t.Errorf("should have two tags, got %v", p.AutoApprovers.ExitNode)
				}
			},
		},
		{
			name:   "nil policy",
			policy: nil,
			tag:    "tag:exitnode",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureAutoApprover(tt.policy, tt.tag)
			if got != tt.want {
				t.Errorf("EnsureAutoApprover() = %v, want %v", got, tt.want)
			}
			if tt.validate != nil && tt.policy != nil {
				tt.validate(t, tt.policy)
			}
		})
	}
}

func TestConfigureForExitNodes(t *testing.T) {
	tests := []struct {
		name          string
		policy        *ACLPolicy
		owner         string
		wantModified  bool
		wantChanges   int // number of change messages
	}{
		{
			name:         "configure empty policy",
			policy:       &ACLPolicy{},
			owner:        "autogroup:admin",
			wantModified: true,
			wantChanges:  2, // tagOwner + autoApprover
		},
		{
			name: "already configured",
			policy: &ACLPolicy{
				TagOwners: map[string][]string{
					"tag:exitnode": {"autogroup:admin"},
				},
				AutoApprovers: &AutoApprovers{
					ExitNode: []string{"tag:exitnode"},
				},
			},
			owner:        "autogroup:admin",
			wantModified: false,
			wantChanges:  2, // status messages
		},
		{
			name: "partial configuration",
			policy: &ACLPolicy{
				TagOwners: map[string][]string{
					"tag:exitnode": {"autogroup:admin"},
				},
			},
			owner:        "autogroup:admin",
			wantModified: true,
			wantChanges:  2, // tagOwner status + autoApprover added
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, modified := ConfigureForExitNodes(tt.policy, tt.owner)
			if modified != tt.wantModified {
				t.Errorf("ConfigureForExitNodes() modified = %v, want %v", modified, tt.wantModified)
			}
			if len(changes) != tt.wantChanges {
				t.Errorf("ConfigureForExitNodes() changes count = %v, want %v\nChanges: %v", len(changes), tt.wantChanges, changes)
			}
		})
	}
}

func TestValidateExitNodeConfig(t *testing.T) {
	tests := []struct {
		name    string
		policy  *ACLPolicy
		wantErr bool
	}{
		{
			name: "valid configuration",
			policy: &ACLPolicy{
				TagOwners: map[string][]string{
					"tag:exitnode": {"autogroup:admin"},
				},
				AutoApprovers: &AutoApprovers{
					ExitNode: []string{"tag:exitnode"},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty policy",
			policy:  &ACLPolicy{},
			wantErr: true,
		},
		{
			name: "missing tagOwners",
			policy: &ACLPolicy{
				AutoApprovers: &AutoApprovers{
					ExitNode: []string{"tag:exitnode"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing autoApprovers",
			policy: &ACLPolicy{
				TagOwners: map[string][]string{
					"tag:exitnode": {"autogroup:admin"},
				},
			},
			wantErr: true,
		},
		{
			name:    "nil policy",
			policy:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExitNodeConfig(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExitNodeConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHasTagOwner(t *testing.T) {
	policy := &ACLPolicy{
		TagOwners: map[string][]string{
			"tag:exitnode": {"autogroup:admin"},
		},
	}

	if !HasTagOwner(policy, "tag:exitnode") {
		t.Error("HasTagOwner() should return true for existing tag")
	}

	if HasTagOwner(policy, "tag:other") {
		t.Error("HasTagOwner() should return false for non-existing tag")
	}

	if HasTagOwner(nil, "tag:exitnode") {
		t.Error("HasTagOwner() should return false for nil policy")
	}

	if HasTagOwner(&ACLPolicy{}, "tag:exitnode") {
		t.Error("HasTagOwner() should return false for policy with nil TagOwners")
	}
}

func TestHasAutoApprover(t *testing.T) {
	policy := &ACLPolicy{
		AutoApprovers: &AutoApprovers{
			ExitNode: []string{"tag:exitnode", "tag:other"},
		},
	}

	if !HasAutoApprover(policy, "tag:exitnode") {
		t.Error("HasAutoApprover() should return true for existing tag")
	}

	if HasAutoApprover(policy, "tag:missing") {
		t.Error("HasAutoApprover() should return false for non-existing tag")
	}

	if HasAutoApprover(nil, "tag:exitnode") {
		t.Error("HasAutoApprover() should return false for nil policy")
	}

	if HasAutoApprover(&ACLPolicy{}, "tag:exitnode") {
		t.Error("HasAutoApprover() should return false for policy with nil AutoApprovers")
	}
}
