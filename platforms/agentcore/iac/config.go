// Package iac provides shared infrastructure-as-code configuration for AgentCore deployments.
//
// This package contains configuration structs and utilities that are shared across
// different IaC tools (CDK, Pulumi, Terraform, CloudFormation). The configuration
// can be defined in Go code, JSON, or YAML files.
//
// Four deployment approaches are supported:
//  1. CDK Go constructs - via github.com/plexusone/agentkit-aws-cdk
//  2. CDK + JSON/YAML config - configuration files with minimal CDK wrapper
//  3. Pulumi - via github.com/plexusone/agentkit-aws-pulumi
//  4. Pure CloudFormation - generate CF templates, deploy with AWS CLI
//
// Example usage:
//
//	config, err := iac.LoadStackConfigFromFile("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use with CDK, Pulumi, or generate CloudFormation
package iac

import (
	"fmt"
)

// AgentConfig defines configuration for a single AgentCore agent.
type AgentConfig struct {
	// Name is the unique identifier for this agent.
	// Used for routing in multi-agent setups.
	Name string `json:"name" yaml:"name"`

	// Description is a human-readable description of the agent.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// ContainerImage is the ECR image URI for the agent.
	// Example: "123456789.dkr.ecr.us-east-1.amazonaws.com/my-agent:latest"
	ContainerImage string `json:"containerImage" yaml:"containerImage"`

	// MemoryMB is the memory allocation in megabytes.
	// Valid values: 512, 1024, 2048, 4096, 8192, 16384
	// Default: 512
	MemoryMB int `json:"memoryMB,omitempty" yaml:"memoryMB,omitempty"`

	// TimeoutSeconds is the maximum execution time.
	// Range: 1-900 (15 minutes max)
	// Default: 300
	TimeoutSeconds int `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`

	// Environment contains environment variables for the agent.
	// API keys should use SecretsARNs instead for security.
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`

	// SecretsARNs is a list of AWS Secrets Manager ARNs to inject.
	// These are mounted as environment variables at runtime.
	SecretsARNs []string `json:"secretsARNs,omitempty" yaml:"secretsARNs,omitempty"`

	// IsDefault marks this as the default agent for the stack.
	// Only one agent should have IsDefault=true.
	IsDefault bool `json:"isDefault,omitempty" yaml:"isDefault,omitempty"`

	// Protocol is the communication protocol for the agent runtime.
	// Supported: "HTTP", "MCP", "A2A"
	// Default: "HTTP"
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`

	// Authorizer configures inbound authorization for the agent.
	// Optional - if not set, no authorization is required.
	Authorizer *AuthorizerConfig `json:"authorizer,omitempty" yaml:"authorizer,omitempty"`

	// EnableMemory enables persistent memory for the agent.
	// Default: false
	EnableMemory bool `json:"enableMemory,omitempty" yaml:"enableMemory,omitempty"`
}

// AuthorizerConfig defines authorization configuration for an agent.
type AuthorizerConfig struct {
	// Type is the authorization type.
	// Supported: "IAM", "LAMBDA", "NONE"
	// Default: "NONE"
	Type string `json:"type" yaml:"type"`

	// LambdaARN is the ARN of the Lambda authorizer function.
	// Required when Type is "LAMBDA".
	LambdaARN string `json:"lambdaArn,omitempty" yaml:"lambdaArn,omitempty"`
}

// VPCConfig defines networking configuration for AgentCore deployment.
type VPCConfig struct {
	// VPCID is an existing VPC to use. If empty, a new VPC is created.
	VPCID string `json:"vpcId,omitempty" yaml:"vpcId,omitempty"`

	// SubnetIDs are existing subnets to use. Required if VPCID is set.
	SubnetIDs []string `json:"subnetIds,omitempty" yaml:"subnetIds,omitempty"`

	// SecurityGroupIDs are existing security groups. Optional.
	SecurityGroupIDs []string `json:"securityGroupIds,omitempty" yaml:"securityGroupIds,omitempty"`

	// CreateVPC creates a new VPC if true. Ignored if VPCID is set.
	// Default: true
	CreateVPC bool `json:"createVPC,omitempty" yaml:"createVPC,omitempty"`

	// VPCCidr is the CIDR block for the new VPC.
	// Default: "10.0.0.0/16"
	VPCCidr string `json:"vpcCidr,omitempty" yaml:"vpcCidr,omitempty"`

	// MaxAZs is the maximum number of availability zones.
	// Default: 2
	MaxAZs int `json:"maxAZs,omitempty" yaml:"maxAZs,omitempty"`

	// EnableVPCEndpoints creates VPC endpoints for AWS services.
	// Reduces NAT Gateway costs and improves security.
	// Default: true
	EnableVPCEndpoints bool `json:"enableVPCEndpoints,omitempty" yaml:"enableVPCEndpoints,omitempty"`
}

// SecretsConfig defines AWS Secrets Manager configuration.
type SecretsConfig struct {
	// CreateSecrets creates new secrets if true.
	// If false, existing secret ARNs must be provided in AgentConfig.SecretsARNs.
	CreateSecrets bool `json:"createSecrets,omitempty" yaml:"createSecrets,omitempty"`

	// SecretValues contains key-value pairs to store as secrets.
	// Keys become environment variable names at runtime.
	// Example: {"GEMINI_API_KEY": "abc123", "OPIK_API_KEY": "xyz789"}
	SecretValues map[string]string `json:"secretValues,omitempty" yaml:"secretValues,omitempty"`

	// SecretName is the name of the secret in Secrets Manager.
	// Default: "{stack-name}-secrets"
	SecretName string `json:"secretName,omitempty" yaml:"secretName,omitempty"`

	// KMSKeyARN is an optional KMS key for encryption.
	// If empty, uses AWS managed key.
	KMSKeyARN string `json:"kmsKeyARN,omitempty" yaml:"kmsKeyARN,omitempty"`
}

// ObservabilityConfig defines monitoring and tracing configuration.
type ObservabilityConfig struct {
	// Provider is the observability provider.
	// Supported: "opik", "langfuse", "phoenix", "cloudwatch"
	// Default: "opik"
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`

	// Project is the project name for grouping traces.
	// Default: stack name
	Project string `json:"project,omitempty" yaml:"project,omitempty"`

	// APIKeySecretARN is the ARN of the secret containing the provider API key.
	// Required for opik, langfuse, phoenix.
	APIKeySecretARN string `json:"apiKeySecretARN,omitempty" yaml:"apiKeySecretARN,omitempty"`

	// Endpoint is a custom endpoint URL (optional).
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`

	// EnableXRay enables AWS X-Ray tracing.
	// Default: false
	EnableXRay bool `json:"enableXRay,omitempty" yaml:"enableXRay,omitempty"`

	// EnableCloudWatchLogs enables CloudWatch Logs.
	// Default: true
	EnableCloudWatchLogs bool `json:"enableCloudWatchLogs,omitempty" yaml:"enableCloudWatchLogs,omitempty"`

	// LogRetentionDays is the CloudWatch Logs retention period.
	// Default: 30
	LogRetentionDays int `json:"logRetentionDays,omitempty" yaml:"logRetentionDays,omitempty"`
}

// IAMConfig defines IAM role and policy configuration.
type IAMConfig struct {
	// RoleARN is an existing IAM role to use.
	// If empty, a new role is created with required permissions.
	RoleARN string `json:"roleARN,omitempty" yaml:"roleARN,omitempty"`

	// AdditionalPolicies are additional IAM policy ARNs to attach.
	AdditionalPolicies []string `json:"additionalPolicies,omitempty" yaml:"additionalPolicies,omitempty"`

	// PermissionsBoundaryARN is an optional permissions boundary.
	PermissionsBoundaryARN string `json:"permissionsBoundaryARN,omitempty" yaml:"permissionsBoundaryARN,omitempty"`

	// EnableBedrockAccess grants access to Bedrock models.
	// Default: true
	EnableBedrockAccess bool `json:"enableBedrockAccess,omitempty" yaml:"enableBedrockAccess,omitempty"`

	// BedrockModelIDs are specific model IDs to allow.
	// If empty, allows all models ("bedrock:*").
	BedrockModelIDs []string `json:"bedrockModelIds,omitempty" yaml:"bedrockModelIds,omitempty"`
}

// GatewayConfig defines configuration for a multi-agent gateway.
type GatewayConfig struct {
	// Enabled enables gateway creation.
	// Default: false
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Name is the gateway name.
	// Default: "{stack-name}-gateway"
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Description is a description of the gateway.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Targets is a list of agent names to route to.
	// If empty, all agents in the stack are included.
	Targets []string `json:"targets,omitempty" yaml:"targets,omitempty"`
}

// StackConfig defines the complete configuration for an AgentCore deployment stack.
type StackConfig struct {
	// StackName is the CloudFormation/CDK stack name.
	// Required.
	StackName string `json:"stackName" yaml:"stackName"`

	// Description is a description for the stack.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Agents is the list of agents to deploy.
	// At least one agent is required.
	Agents []AgentConfig `json:"agents" yaml:"agents"`

	// VPC configures networking.
	// Optional - uses sensible defaults if not provided.
	VPC *VPCConfig `json:"vpc,omitempty" yaml:"vpc,omitempty"`

	// Secrets configures AWS Secrets Manager.
	// Optional.
	Secrets *SecretsConfig `json:"secrets,omitempty" yaml:"secrets,omitempty"`

	// Observability configures monitoring and tracing.
	// Optional - defaults to Opik with CloudWatch Logs.
	Observability *ObservabilityConfig `json:"observability,omitempty" yaml:"observability,omitempty"`

	// IAM configures IAM roles and policies.
	// Optional - creates required roles automatically.
	IAM *IAMConfig `json:"iam,omitempty" yaml:"iam,omitempty"`

	// Gateway configures a multi-agent gateway for routing.
	// Optional - only needed for multi-agent communication.
	Gateway *GatewayConfig `json:"gateway,omitempty" yaml:"gateway,omitempty"`

	// Tags are AWS resource tags applied to all resources.
	Tags map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`

	// RemovalPolicy determines what happens to resources on stack deletion.
	// "destroy" removes all resources, "retain" keeps them.
	// Default: "destroy"
	RemovalPolicy string `json:"removalPolicy,omitempty" yaml:"removalPolicy,omitempty"`
}

// DefaultAgentConfig returns an AgentConfig with sensible defaults.
func DefaultAgentConfig(name, containerImage string) AgentConfig {
	return AgentConfig{
		Name:           name,
		Description:    fmt.Sprintf("AgentCore agent: %s", name),
		ContainerImage: containerImage,
		MemoryMB:       512,
		TimeoutSeconds: 300,
		Environment:    make(map[string]string),
		SecretsARNs:    []string{},
	}
}

// DefaultVPCConfig returns a VPCConfig with sensible defaults.
func DefaultVPCConfig() *VPCConfig {
	return &VPCConfig{
		CreateVPC:          true,
		VPCCidr:            "10.0.0.0/16",
		MaxAZs:             2,
		EnableVPCEndpoints: true,
	}
}

// DefaultObservabilityConfig returns an ObservabilityConfig with sensible defaults.
func DefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		Provider:             "opik",
		EnableXRay:           false,
		EnableCloudWatchLogs: true,
		LogRetentionDays:     30,
	}
}

// DefaultIAMConfig returns an IAMConfig with sensible defaults.
func DefaultIAMConfig() *IAMConfig {
	return &IAMConfig{
		EnableBedrockAccess: true,
		BedrockModelIDs:     []string{}, // Allow all models
	}
}

// Validate validates the StackConfig and returns any errors.
func (c *StackConfig) Validate() error {
	if c.StackName == "" {
		return fmt.Errorf("stackName is required")
	}

	if len(c.Agents) == 0 {
		return fmt.Errorf("at least one agent is required")
	}

	defaultCount := 0
	agentNames := make(map[string]bool)

	for i, agent := range c.Agents {
		if agent.Name == "" {
			return fmt.Errorf("agents[%d]: name is required", i)
		}
		if agent.ContainerImage == "" {
			return fmt.Errorf("agents[%d] (%s): containerImage is required", i, agent.Name)
		}
		if agentNames[agent.Name] {
			return fmt.Errorf("duplicate agent name: %s", agent.Name)
		}
		agentNames[agent.Name] = true

		if agent.IsDefault {
			defaultCount++
		}

		if agent.MemoryMB != 0 {
			validMemory := []int{512, 1024, 2048, 4096, 8192, 16384}
			valid := false
			for _, m := range validMemory {
				if agent.MemoryMB == m {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("agents[%d] (%s): memoryMB must be one of %v", i, agent.Name, validMemory)
			}
		}

		if agent.TimeoutSeconds != 0 && (agent.TimeoutSeconds < 1 || agent.TimeoutSeconds > 900) {
			return fmt.Errorf("agents[%d] (%s): timeoutSeconds must be between 1 and 900", i, agent.Name)
		}

		// Validate protocol
		if agent.Protocol != "" {
			validProtocols := []string{"HTTP", "MCP", "A2A"}
			valid := false
			for _, p := range validProtocols {
				if agent.Protocol == p {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("agents[%d] (%s): protocol must be one of %v", i, agent.Name, validProtocols)
			}
		}

		// Validate authorizer
		if agent.Authorizer != nil {
			validAuthTypes := []string{"IAM", "LAMBDA", "NONE"}
			valid := false
			for _, t := range validAuthTypes {
				if agent.Authorizer.Type == t {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("agents[%d] (%s): authorizer.type must be one of %v", i, agent.Name, validAuthTypes)
			}
			if agent.Authorizer.Type == "LAMBDA" && agent.Authorizer.LambdaARN == "" {
				return fmt.Errorf("agents[%d] (%s): authorizer.lambdaArn is required when type is LAMBDA", i, agent.Name)
			}
		}
	}

	if defaultCount > 1 {
		return fmt.Errorf("only one agent can be marked as default")
	}

	// Validate gateway targets reference existing agents
	if c.Gateway != nil && c.Gateway.Enabled && len(c.Gateway.Targets) > 0 {
		for _, target := range c.Gateway.Targets {
			if !agentNames[target] {
				return fmt.Errorf("gateway target '%s' does not match any agent name", target)
			}
		}
	}

	if c.VPC != nil && c.VPC.VPCID != "" && len(c.VPC.SubnetIDs) == 0 {
		return fmt.Errorf("vpc.subnetIds are required when using an existing VPC")
	}

	if c.Observability != nil && c.Observability.Provider != "" {
		validProviders := []string{"opik", "langfuse", "phoenix", "cloudwatch"}
		valid := false
		for _, p := range validProviders {
			if c.Observability.Provider == p {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid observability.provider: %s (valid: %v)", c.Observability.Provider, validProviders)
		}
	}

	return nil
}

// ApplyDefaults applies default values to unset fields.
func (c *StackConfig) ApplyDefaults() {
	if c.Description == "" {
		c.Description = fmt.Sprintf("AgentCore stack: %s", c.StackName)
	}

	if c.VPC == nil {
		c.VPC = DefaultVPCConfig()
	}

	if c.Observability == nil {
		c.Observability = DefaultObservabilityConfig()
	}
	if c.Observability.Project == "" {
		c.Observability.Project = c.StackName
	}

	if c.IAM == nil {
		c.IAM = DefaultIAMConfig()
	}

	if c.RemovalPolicy == "" {
		c.RemovalPolicy = "destroy"
	}

	if c.Tags == nil {
		c.Tags = make(map[string]string)
	}
	if _, ok := c.Tags["ManagedBy"]; !ok {
		c.Tags["ManagedBy"] = "agentkit"
	}

	// Apply gateway defaults
	if c.Gateway != nil && c.Gateway.Enabled {
		if c.Gateway.Name == "" {
			c.Gateway.Name = fmt.Sprintf("%s-gateway", c.StackName)
		}
		if c.Gateway.Description == "" {
			c.Gateway.Description = fmt.Sprintf("Gateway for %s", c.StackName)
		}
	}

	for i := range c.Agents {
		if c.Agents[i].MemoryMB == 0 {
			c.Agents[i].MemoryMB = 512
		}
		if c.Agents[i].TimeoutSeconds == 0 {
			c.Agents[i].TimeoutSeconds = 300
		}
		if c.Agents[i].Description == "" {
			c.Agents[i].Description = fmt.Sprintf("AgentCore agent: %s", c.Agents[i].Name)
		}
		if c.Agents[i].Environment == nil {
			c.Agents[i].Environment = make(map[string]string)
		}
		if c.Agents[i].Protocol == "" {
			c.Agents[i].Protocol = "HTTP"
		}
	}
}

// ValidMemoryValues returns the list of valid memory values in MB.
func ValidMemoryValues() []int {
	return []int{512, 1024, 2048, 4096, 8192, 16384}
}

// ValidObservabilityProviders returns the list of valid observability providers.
func ValidObservabilityProviders() []string {
	return []string{"opik", "langfuse", "phoenix", "cloudwatch"}
}

// ValidProtocols returns the list of valid agent protocols.
func ValidProtocols() []string {
	return []string{"HTTP", "MCP", "A2A"}
}

// ValidAuthorizerTypes returns the list of valid authorizer types.
func ValidAuthorizerTypes() []string {
	return []string{"IAM", "LAMBDA", "NONE"}
}
