package tailscale

import (
	"fmt"
	"strings"
)

// EnsureTagOwner adds a tag to tagOwners if not present
// Returns true if changes were made
func EnsureTagOwner(policy *ACLPolicy, tag string, owner string) bool {
	if policy == nil {
		return false
	}

	// Initialize TagOwners if nil
	if policy.TagOwners == nil {
		policy.TagOwners = make(map[string][]string)
	}

	// Check if tag already exists
	if owners, exists := policy.TagOwners[tag]; exists {
		// Check if owner already in the list
		for _, o := range owners {
			if o == owner {
				return false // Already configured
			}
		}
		// Tag exists but owner is different - don't modify
		return false
	}

	// Add new tag with owner
	policy.TagOwners[tag] = []string{owner}
	return true
}

// EnsureAutoApprover adds a tag to exitNode autoApprovers if not present
// Returns true if changes were made
func EnsureAutoApprover(policy *ACLPolicy, tag string) bool {
	if policy == nil {
		return false
	}

	// Initialize AutoApprovers if nil
	if policy.AutoApprovers == nil {
		policy.AutoApprovers = &AutoApprovers{}
	}

	// Check if tag already in exitNode list
	for _, approver := range policy.AutoApprovers.ExitNode {
		if approver == tag {
			return false // Already configured
		}
	}

	// Add tag to exitNode approvers
	policy.AutoApprovers.ExitNode = append(policy.AutoApprovers.ExitNode, tag)
	return true
}

// HasTagOwner checks if a tag exists in tagOwners
func HasTagOwner(policy *ACLPolicy, tag string) bool {
	if policy == nil || policy.TagOwners == nil {
		return false
	}
	_, exists := policy.TagOwners[tag]
	return exists
}

// GetTagOwners returns the owners for a specific tag
func GetTagOwners(policy *ACLPolicy, tag string) []string {
	if policy == nil || policy.TagOwners == nil {
		return nil
	}
	return policy.TagOwners[tag]
}

// HasAutoApprover checks if a tag is in exitNode autoApprovers
func HasAutoApprover(policy *ACLPolicy, tag string) bool {
	if policy == nil || policy.AutoApprovers == nil {
		return false
	}

	for _, approver := range policy.AutoApprovers.ExitNode {
		if approver == tag {
			return true
		}
	}
	return false
}

// ConfigureForExitNodes configures ACL policy for TSE exit nodes
// Returns a list of changes made and a boolean indicating if changes were applied
func ConfigureForExitNodes(policy *ACLPolicy, owner string) ([]string, bool) {
	if policy == nil {
		return nil, false
	}

	var changes []string
	modified := false

	// Ensure tag:exitnode in tagOwners
	if EnsureTagOwner(policy, "tag:exitnode", owner) {
		changes = append(changes, fmt.Sprintf("Added tag:exitnode to tagOwners (owner: %s)", owner))
		modified = true
	} else if HasTagOwner(policy, "tag:exitnode") {
		owners := GetTagOwners(policy, "tag:exitnode")
		changes = append(changes, fmt.Sprintf("✓ tag:exitnode already in tagOwners (owners: %s)", strings.Join(owners, ", ")))
	}

	// Ensure tag:exitnode in autoApprovers.exitNode
	if EnsureAutoApprover(policy, "tag:exitnode") {
		changes = append(changes, "Added tag:exitnode to exit node auto-approvers")
		modified = true
	} else if HasAutoApprover(policy, "tag:exitnode") {
		changes = append(changes, "✓ tag:exitnode already in exit node auto-approvers")
	}

	return changes, modified
}

// PreviewChanges returns a human-readable description of what would change
func PreviewChanges(current *ACLPolicy, owner string) []string {
	if current == nil {
		return []string{"Error: current policy is nil"}
	}

	var preview []string

	// Check tagOwners
	if !HasTagOwner(current, "tag:exitnode") {
		preview = append(preview, fmt.Sprintf("+ Add tag:exitnode to tagOwners with owner: %s", owner))
	} else {
		owners := GetTagOwners(current, "tag:exitnode")
		preview = append(preview, fmt.Sprintf("  tag:exitnode already exists in tagOwners (owners: %s)", strings.Join(owners, ", ")))
	}

	// Check autoApprovers
	if !HasAutoApprover(current, "tag:exitnode") {
		preview = append(preview, "+ Add tag:exitnode to exit node auto-approvers")
	} else {
		preview = append(preview, "  tag:exitnode already in exit node auto-approvers")
	}

	return preview
}

// ValidateExitNodeConfig checks if ACL is properly configured for exit nodes
// Returns nil if properly configured, error describing what's missing otherwise
func ValidateExitNodeConfig(policy *ACLPolicy) error {
	if policy == nil {
		return fmt.Errorf("ACL policy is nil")
	}

	var missing []string

	if !HasTagOwner(policy, "tag:exitnode") {
		missing = append(missing, "tag:exitnode not in tagOwners")
	}

	if !HasAutoApprover(policy, "tag:exitnode") {
		missing = append(missing, "tag:exitnode not in exit node auto-approvers")
	}

	if len(missing) > 0 {
		return fmt.Errorf("ACL not configured for exit nodes: %s", strings.Join(missing, ", "))
	}

	return nil
}
