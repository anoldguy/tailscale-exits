package infrastructure

// Resource represents an AWS resource with basic identifying information.
// Used for resources that share the same structure (IAM Role, Lambda Function, Log Group).
type Resource struct {
	Name string
	ARN  string
	Tags map[string]string
}

// InfrastructureState represents the discovered state of TSE AWS infrastructure.
// All resources are discovered via tags (ManagedBy=tse) with no local state file.
type InfrastructureState struct {
	LogGroup    *Resource
	IAMRole     *Resource
	Lambda      *Resource
	FunctionURL string // Just the URL string, no need for separate type
	Policies    struct {
		Managed        bool   // Whether AWSLambdaBasicExecutionRole is attached
		InlineName     string // Name of inline policy
		InlineDocument string // Inline policy document
	}
}

// Exists returns true if at least one infrastructure resource was found.
func (s *InfrastructureState) Exists() bool {
	return s.LogGroup != nil || s.IAMRole != nil || s.Lambda != nil
}

// IsComplete returns true if all required infrastructure is deployed.
func (s *InfrastructureState) IsComplete() bool {
	return s.LogGroup != nil &&
		s.IAMRole != nil &&
		s.Lambda != nil &&
		s.FunctionURL != "" &&
		s.Policies.Managed &&
		s.Policies.InlineName != ""
}

// Missing returns a list of resources that are not yet deployed.
func (s *InfrastructureState) Missing() []string {
	var missing []string
	if s.LogGroup == nil {
		missing = append(missing, "CloudWatch Log Group")
	}
	if s.IAMRole == nil {
		missing = append(missing, "IAM Role")
	}
	if !s.Policies.Managed {
		missing = append(missing, "Managed Policy Attachment")
	}
	if s.Policies.InlineName == "" {
		missing = append(missing, "Inline Policy")
	}
	if s.Lambda == nil {
		missing = append(missing, "Lambda Function")
	}
	if s.FunctionURL == "" {
		missing = append(missing, "Function URL")
	}
	return missing
}

// HasOnlyIAMResources returns true if only IAM resources exist (role/policies)
// but no regional resources (Lambda, logs).
// This indicates the user might be checking the wrong region.
func (s *InfrastructureState) HasOnlyIAMResources() bool {
	hasIAM := s.IAMRole != nil || s.Policies.Managed || s.Policies.InlineName != ""
	hasRegional := s.LogGroup != nil || s.Lambda != nil || s.FunctionURL != ""
	return hasIAM && !hasRegional
}
