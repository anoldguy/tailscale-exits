package regions

import (
	"fmt"
	"strings"
)

// friendlyToAWS maps human-friendly region names to AWS region codes
var friendlyToAWS = map[string]string{
	"ohio":       "us-east-2",
	"virginia":   "us-east-1",
	"oregon":     "us-west-2",
	"california": "us-west-1",
	"canada":     "ca-central-1",
	"ireland":    "eu-west-1",
	"london":     "eu-west-2",
	"paris":      "eu-west-3",
	"frankfurt":  "eu-central-1",
	"stockholm":  "eu-north-1",
	"singapore":  "ap-southeast-1",
	"sydney":     "ap-southeast-2",
	"tokyo":      "ap-northeast-1",
	"seoul":      "ap-northeast-2",
	"mumbai":     "ap-south-1",
	"saopaulo":   "sa-east-1",
}

// awsToFriendly maps AWS region codes to human-friendly names
var awsToFriendly = map[string]string{}

func init() {
	// Build reverse mapping
	for friendly, aws := range friendlyToAWS {
		awsToFriendly[aws] = friendly
	}
}

// GetAWSRegion converts a friendly region name to AWS region code
// Returns error if the friendly name is not recognized
func GetAWSRegion(friendlyName string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(friendlyName))
	awsRegion, ok := friendlyToAWS[normalized]
	if !ok {
		return "", fmt.Errorf("unknown region '%s'. Available regions: %s", friendlyName, GetAvailableRegions())
	}
	return awsRegion, nil
}

// GetFriendlyName converts an AWS region code to a friendly name
// Returns error if the AWS region is not recognized
func GetFriendlyName(awsRegion string) (string, error) {
	friendlyName, ok := awsToFriendly[awsRegion]
	if !ok {
		return "", fmt.Errorf("unknown AWS region '%s'", awsRegion)
	}
	return friendlyName, nil
}

// GetAvailableRegions returns a comma-separated list of available friendly region names
func GetAvailableRegions() string {
	regions := make([]string, 0, len(friendlyToAWS))
	for friendly := range friendlyToAWS {
		regions = append(regions, friendly)
	}
	return strings.Join(regions, ", ")
}

// GetAllFriendlyNames returns a slice of all available friendly region names
func GetAllFriendlyNames() []string {
	regions := make([]string, 0, len(friendlyToAWS))
	for friendly := range friendlyToAWS {
		regions = append(regions, friendly)
	}
	return regions
}

// IsValidFriendlyName checks if a friendly name is supported
func IsValidFriendlyName(friendlyName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(friendlyName))
	_, ok := friendlyToAWS[normalized]
	return ok
}

// IsValidAWSRegion checks if an AWS region code is supported
func IsValidAWSRegion(awsRegion string) bool {
	_, ok := awsToFriendly[awsRegion]
	return ok
}
