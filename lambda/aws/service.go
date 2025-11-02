package aws

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/anoldguy/tse/shared/regions"
	sharedtypes "github.com/anoldguy/tse/shared/types"
)

const (
	// InstanceType is the ARM instance type we use for cost efficiency
	InstanceType = "t4g.nano"

	// SecurityGroupName is the name for our ephemeral security group
	SecurityGroupName = "tse-ephemeral-exit-node"

	// TagProject is the tag key for identifying our resources
	TagProject = "tse"

	// TagType is the tag value for our ephemeral resources
	TagType = "ephemeral"
)

// Service provides AWS operations for the exit node service
type Service struct {
	ec2Client *ec2.Client
}

// New creates a new AWS service instance
func New(ctx context.Context, region string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Service{
		ec2Client: ec2.NewFromConfig(cfg),
	}, nil
}

// userDataTemplate defines the bash script for Tailscale installation
const userDataTemplate = `#!/bin/bash
set -e

# Install Tailscale
curl -fsSL https://tailscale.com/install.sh | sh

# Start Tailscale with exit node advertisement
tailscale up --authkey={{.AuthKey}} --advertise-exit-node --hostname=exit-{{.Region}}

# Enable IP forwarding
echo 'net.ipv4.ip_forward = 1' >> /etc/sysctl.conf
echo 'net.ipv6.conf.all.forwarding = 1' >> /etc/sysctl.conf
sysctl -p

# Log completion
echo "Tailscale exit node setup complete for region: {{.Region}}" | logger -t tse-setup
`

var userDataTmpl = template.Must(template.New("userdata").Parse(userDataTemplate))

// generateUserData creates the user data script for Tailscale installation
func generateUserData(authKey, friendlyRegion string) string {
	var buf bytes.Buffer
	err := userDataTmpl.Execute(&buf, map[string]string{
		"AuthKey": authKey,
		"Region":  friendlyRegion,
	})
	if err != nil {
		// Template execution should never fail with a constant template
		panic(fmt.Sprintf("failed to execute user data template: %v", err))
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// findOrCreateSecurityGroup ensures our security group exists with proper rules in the specified VPC
func (s *Service) findOrCreateSecurityGroup(ctx context.Context, vpcID, friendlyRegion string) (string, error) {
	// Try to find existing security group in the VPC
	result, err := s.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("tag:Project"),
				Values: []string{TagProject},
			},
			{
				Name:   aws.String("tag:Type"),
				Values: []string{TagType},
			},
			{
				Name:   aws.String("tag:Region"),
				Values: []string{friendlyRegion},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe security groups: %w", err)
	}

	if len(result.SecurityGroups) > 0 {
		return *result.SecurityGroups[0].GroupId, nil
	}

	// Create new security group in the VPC
	createResult, err := s.ec2Client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(fmt.Sprintf("tse-sg-%s", friendlyRegion)),
		Description: aws.String("Tailscale ephemeral exit node security group"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("tse-sg-%s", friendlyRegion))},
					{Key: aws.String("Project"), Value: aws.String(TagProject)},
					{Key: aws.String("Type"), Value: aws.String(TagType)},
					{Key: aws.String("Region"), Value: aws.String(friendlyRegion)},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}

	sgID := *createResult.GroupId

	// Add inbound rules for WireGuard and SSH (temporary for debugging)
	_, err = s.ec2Client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("udp"),
				FromPort:   aws.Int32(41641),
				ToPort:     aws.Int32(41641),
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				},
			},
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to add security group rules: %w", err)
	}

	return sgID, nil
}

// getLatestAmazonLinux2023ARM64AMI finds the latest Amazon Linux 2023 ARM64 AMI
func (s *Service) getLatestAmazonLinux2023ARM64AMI(ctx context.Context) (string, error) {
	result, err := s.ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"al2023-ami-*-arm64"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"arm64"},
			},
		},
	})
	if err != nil {
		return "", err
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf("no Amazon Linux 2023 ARM64 AMI found")
	}

	// Find the most recent AMI
	var latestAMI types.Image
	var latestTime time.Time

	for _, image := range result.Images {
		if image.CreationDate != nil {
			creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
			if err != nil {
				continue
			}
			if creationTime.After(latestTime) {
				latestTime = creationTime
				latestAMI = image
			}
		}
	}

	if latestAMI.ImageId == nil {
		return "", fmt.Errorf("could not determine latest Amazon Linux 2023 ARM64 AMI")
	}

	return *latestAMI.ImageId, nil
}

// findOrCreateVPCStack finds existing TSE VPC infrastructure or creates it
// Returns (subnetID, vpcID, error)
func (s *Service) findOrCreateVPCStack(ctx context.Context, friendlyRegion string) (string, string, error) {
	// First, try to find existing TSE VPC
	vpcResult, err := s.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Project"),
				Values: []string{TagProject},
			},
			{
				Name:   aws.String("tag:Type"),
				Values: []string{TagType},
			},
			{
				Name:   aws.String("tag:Region"),
				Values: []string{friendlyRegion},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to search for existing VPC: %w", err)
	}

	if len(vpcResult.Vpcs) > 0 {
		// Found existing VPC, find its subnet
		vpcID := *vpcResult.Vpcs[0].VpcId
		subnetID, err := s.findSubnetInVPC(ctx, vpcID)
		return subnetID, vpcID, err
	}

	// No existing VPC, create the full stack
	return s.createVPCStack(ctx, friendlyRegion)
}

// findSubnetInVPC finds a subnet in the specified VPC
func (s *Service) findSubnetInVPC(ctx context.Context, vpcID string) (string, error) {
	subnetResult, err := s.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("tag:Project"),
				Values: []string{TagProject},
			},
			{
				Name:   aws.String("tag:Type"),
				Values: []string{TagType},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to find subnets in VPC %s: %w", vpcID, err)
	}

	if len(subnetResult.Subnets) == 0 {
		return "", fmt.Errorf("no TSE subnets found in VPC %s", vpcID)
	}

	return *subnetResult.Subnets[0].SubnetId, nil
}

// createVPCStack creates a complete VPC infrastructure stack
// Returns (subnetID, vpcID, error)
func (s *Service) createVPCStack(ctx context.Context, friendlyRegion string) (string, string, error) {
	// Create VPC
	vpcResult, err := s.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("tse-vpc-%s", friendlyRegion))},
					{Key: aws.String("Project"), Value: aws.String(TagProject)},
					{Key: aws.String("Type"), Value: aws.String(TagType)},
					{Key: aws.String("Region"), Value: aws.String(friendlyRegion)},
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create VPC: %w", err)
	}

	vpcID := *vpcResult.Vpc.VpcId

	// Get first available AZ
	azResult, err := s.ec2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to get availability zones: %w", err)
	}

	if len(azResult.AvailabilityZones) == 0 {
		return "", "", fmt.Errorf("no available availability zones found")
	}

	azName := *azResult.AvailabilityZones[0].ZoneName

	// Create subnet
	subnetResult, err := s.ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcID),
		CidrBlock:        aws.String("10.0.1.0/24"),
		AvailabilityZone: aws.String(azName),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("tse-subnet-%s", friendlyRegion))},
					{Key: aws.String("Project"), Value: aws.String(TagProject)},
					{Key: aws.String("Type"), Value: aws.String(TagType)},
					{Key: aws.String("Region"), Value: aws.String(friendlyRegion)},
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create subnet: %w", err)
	}

	subnetID := *subnetResult.Subnet.SubnetId

	// Create Internet Gateway
	igwResult, err := s.ec2Client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInternetGateway,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("tse-igw-%s", friendlyRegion))},
					{Key: aws.String("Project"), Value: aws.String(TagProject)},
					{Key: aws.String("Type"), Value: aws.String(TagType)},
					{Key: aws.String("Region"), Value: aws.String(friendlyRegion)},
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create internet gateway: %w", err)
	}

	igwID := *igwResult.InternetGateway.InternetGatewayId

	// Attach Internet Gateway to VPC
	_, err = s.ec2Client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to attach internet gateway: %w", err)
	}

	// Get the route table for the VPC
	rtResult, err := s.ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to find route table: %w", err)
	}

	if len(rtResult.RouteTables) == 0 {
		return "", "", fmt.Errorf("no route table found for VPC")
	}

	routeTableID := *rtResult.RouteTables[0].RouteTableId

	// Add route to Internet Gateway
	_, err = s.ec2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(igwID),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create route to internet gateway: %w", err)
	}

	// Enable auto-assign public IP for the subnet
	_, err = s.ec2Client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
		SubnetId: aws.String(subnetID),
		MapPublicIpOnLaunch: &types.AttributeBooleanValue{
			Value: aws.Bool(true),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to enable auto-assign public IP for subnet: %w", err)
	}

	return subnetID, vpcID, nil
}

// StartInstance creates a new exit node instance
func (s *Service) StartInstance(ctx context.Context, friendlyRegion, authKey string) (*sharedtypes.InstanceInfo, error) {
	awsRegion, err := regions.GetAWSRegion(friendlyRegion)
	if err != nil {
		return nil, err
	}

	// Get latest Amazon Linux 2023 ARM64 AMI
	amiID, err := s.getLatestAmazonLinux2023ARM64AMI(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find Amazon Linux 2023 ARM64 AMI: %w", err)
	}

	// Find or create VPC infrastructure
	subnetID, vpcID, err := s.findOrCreateVPCStack(ctx, friendlyRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to setup VPC infrastructure: %w", err)
	}

	// Ensure security group exists in the VPC
	sgID, err := s.findOrCreateSecurityGroup(ctx, vpcID, friendlyRegion)
	if err != nil {
		return nil, err
	}

	// Generate user data script
	userData := generateUserData(authKey, friendlyRegion)

	// Launch instance
	runResult, err := s.ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(amiID),
		InstanceType:     types.InstanceType(InstanceType),
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		SubnetId:         aws.String(subnetID),
		SecurityGroupIds: []string{sgID},
		KeyName:          aws.String("tailscale"), // Temporary for debugging
		UserData:         aws.String(userData),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("tse-exit-%s", friendlyRegion))},
					{Key: aws.String("Project"), Value: aws.String(TagProject)},
					{Key: aws.String("Type"), Value: aws.String(TagType)},
					{Key: aws.String("Region"), Value: aws.String(friendlyRegion)},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch instance: %w", err)
	}

	instance := runResult.Instances[0]

	return &sharedtypes.InstanceInfo{
		InstanceID:        *instance.InstanceId,
		Region:            awsRegion,
		FriendlyRegion:    friendlyRegion,
		State:             string(instance.State.Name),
		LaunchTime:        *instance.LaunchTime,
		InstanceType:      string(instance.InstanceType),
		TailscaleHostname: fmt.Sprintf("exit-%s", friendlyRegion),
	}, nil
}

// ListInstances returns all ephemeral exit node instances in the region
func (s *Service) ListInstances(ctx context.Context) ([]*sharedtypes.InstanceInfo, error) {
	result, err := s.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Project"),
				Values: []string{TagProject},
			},
			{
				Name:   aws.String("tag:Type"),
				Values: []string{TagType},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []string{
					"pending",
					"running",
					"stopping",
					"stopped",
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []*sharedtypes.InstanceInfo
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			friendlyRegion := ""
			for _, tag := range instance.Tags {
				if *tag.Key == "Region" {
					friendlyRegion = *tag.Value
					break
				}
			}

			info := &sharedtypes.InstanceInfo{
				InstanceID:     *instance.InstanceId,
				State:          string(instance.State.Name),
				LaunchTime:     *instance.LaunchTime,
				InstanceType:   string(instance.InstanceType),
				FriendlyRegion: friendlyRegion,
			}

			if instance.PublicIpAddress != nil {
				info.PublicIP = *instance.PublicIpAddress
			}
			if instance.PrivateIpAddress != nil {
				info.PrivateIP = *instance.PrivateIpAddress
			}
			if friendlyRegion != "" {
				info.TailscaleHostname = fmt.Sprintf("exit-%s", friendlyRegion)
			}

			instances = append(instances, info)
		}
	}

	return instances, nil
}

// StopInstances terminates all ephemeral exit node instances in the region
func (s *Service) StopInstances(ctx context.Context) ([]string, error) {
	instances, err := s.ListInstances(ctx)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return []string{}, nil
	}

	var instanceIDs []string
	for _, instance := range instances {
		if instance.State == "running" || instance.State == "pending" || instance.State == "stopped" {
			instanceIDs = append(instanceIDs, instance.InstanceID)
		}
	}

	if len(instanceIDs) == 0 {
		return []string{}, nil
	}

	_, err = s.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to terminate instances: %w", err)
	}

	// Wait for instances to be terminated, then clean up VPC infrastructure
	go func() {
		// Give instances time to terminate
		time.Sleep(30 * time.Second)
		s.cleanupVPCInfrastructure(ctx)
	}()

	return instanceIDs, nil
}

// cleanupVPCInfrastructure removes VPC infrastructure when no instances are running
func (s *Service) cleanupVPCInfrastructure(ctx context.Context) error {
	// Check if any TSE instances are still running
	instances, err := s.ListInstances(ctx)
	if err != nil {
		return err
	}

	// If there are still running instances, don't clean up
	for _, instance := range instances {
		if instance.State == "running" || instance.State == "pending" {
			return nil
		}
	}

	// Find TSE VPCs
	vpcResult, err := s.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Project"),
				Values: []string{TagProject},
			},
			{
				Name:   aws.String("tag:Type"),
				Values: []string{TagType},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to find TSE VPCs: %w", err)
	}

	for _, vpc := range vpcResult.Vpcs {
		vpcID := *vpc.VpcId
		if err := s.deleteVPCStack(ctx, vpcID); err != nil {
			// Log error but continue with other VPCs
			fmt.Printf("Failed to delete VPC %s: %v\n", vpcID, err)
		}
	}

	return nil
}

// deleteVPCStack removes a VPC and all its associated infrastructure
func (s *Service) deleteVPCStack(ctx context.Context, vpcID string) error {
	// Delete Internet Gateways
	igwResult, err := s.ec2Client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err == nil {
		for _, igw := range igwResult.InternetGateways {
			igwID := *igw.InternetGatewayId

			// Detach from VPC
			s.ec2Client.DetachInternetGateway(ctx, &ec2.DetachInternetGatewayInput{
				InternetGatewayId: aws.String(igwID),
				VpcId:             aws.String(vpcID),
			})

			// Delete Internet Gateway
			s.ec2Client.DeleteInternetGateway(ctx, &ec2.DeleteInternetGatewayInput{
				InternetGatewayId: aws.String(igwID),
			})
		}
	}

	// Delete Subnets
	subnetResult, err := s.ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})
	if err == nil {
		for _, subnet := range subnetResult.Subnets {
			s.ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
				SubnetId: aws.String(*subnet.SubnetId),
			})
		}
	}

	// Delete VPC
	_, err = s.ec2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete VPC %s: %w", vpcID, err)
	}

	return nil
}

// ForceCleanupAllResources aggressively cleans up all TSE resources in a region
func (s *Service) ForceCleanupAllResources(ctx context.Context, friendlyRegion string) ([]string, error) {
	var cleanedResources []string

	// 1. Terminate all TSE instances
	instances, err := s.ListInstances(ctx)
	if err == nil {
		for _, instance := range instances {
			if instance.State == "running" || instance.State == "pending" || instance.State == "stopped" {
				_, err := s.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
					InstanceIds: []string{instance.InstanceID},
				})
				if err == nil {
					cleanedResources = append(cleanedResources, fmt.Sprintf("Instance:%s", instance.InstanceID))
				}
			}
		}
	}

	// Wait a bit for instances to start terminating
	time.Sleep(5 * time.Second)

	// 2. Delete security groups
	sgResult, err := s.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Project"),
				Values: []string{TagProject},
			},
			{
				Name:   aws.String("tag:Type"),
				Values: []string{TagType},
			},
			{
				Name:   aws.String("tag:Region"),
				Values: []string{friendlyRegion},
			},
		},
	})
	if err == nil {
		for _, sg := range sgResult.SecurityGroups {
			sgID := *sg.GroupId
			_, err := s.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
				GroupId: aws.String(sgID),
			})
			if err == nil {
				cleanedResources = append(cleanedResources, fmt.Sprintf("SecurityGroup:%s", sgID))
			}
		}
	}

	// 3. Clean up VPC infrastructure
	if err := s.cleanupVPCInfrastructure(ctx); err == nil {
		// Find and report VPCs that were cleaned up
		vpcResult, err := s.ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:Project"),
					Values: []string{TagProject},
				},
				{
					Name:   aws.String("tag:Type"),
					Values: []string{TagType},
				},
				{
					Name:   aws.String("tag:Region"),
					Values: []string{friendlyRegion},
				},
			},
		})
		if err == nil {
			for _, vpc := range vpcResult.Vpcs {
				cleanedResources = append(cleanedResources, fmt.Sprintf("VPC:%s", *vpc.VpcId))
			}
		}
	}

	return cleanedResources, nil
}
