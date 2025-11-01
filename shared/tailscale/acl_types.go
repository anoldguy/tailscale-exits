package tailscale

// ACLPolicy represents a Tailscale ACL policy
// This supports both HuJSON (with comments) and standard JSON
type ACLPolicy struct {
	// Groups define named groups of users/devices
	Groups map[string][]string `json:"groups,omitempty"`

	// TagOwners defines who can apply which tags
	TagOwners map[string][]string `json:"tagOwners,omitempty"`

	// AutoApprovers defines automatic approval rules
	AutoApprovers *AutoApprovers `json:"autoApprovers,omitempty"`

	// ACLs define the access control rules
	ACLs []ACLRule `json:"acls,omitempty"`

	// Hosts defines host aliases
	Hosts map[string]string `json:"hosts,omitempty"`

	// Tests define ACL policy tests
	Tests []ACLTest `json:"tests,omitempty"`

	// SSH defines SSH access rules
	SSH []SSHRule `json:"ssh,omitempty"`
}

// AutoApprovers defines resources that are automatically approved
type AutoApprovers struct {
	// Routes defines which devices can advertise which routes
	Routes map[string][]string `json:"routes,omitempty"`

	// ExitNode defines which devices can advertise as exit nodes
	ExitNode []string `json:"exitNode,omitempty"`
}

// ACLRule defines an access control rule
type ACLRule struct {
	// Action is typically "accept"
	Action string `json:"action"`

	// Src defines source users, groups, or tags
	Src []string `json:"src"`

	// Dst defines destination hosts and ports
	Dst []string `json:"dst"`

	// Proto optionally restricts protocol
	Proto string `json:"proto,omitempty"`
}

// ACLTest defines a test case for ACL validation
type ACLTest struct {
	Src    string   `json:"src"`
	Accept []string `json:"accept,omitempty"`
	Deny   []string `json:"deny,omitempty"`
}

// SSHRule defines SSH access rules
type SSHRule struct {
	Action      string   `json:"action"`
	Src         []string `json:"src"`
	Dst         []string `json:"dst"`
	Users       []string `json:"users"`
	CheckPeriod string   `json:"checkPeriod,omitempty"`
}

// ACLResponse represents the response when fetching or updating ACL
type ACLResponse struct {
	ACL  *ACLPolicy
	ETag string // Stored separately from the ACL body
}
