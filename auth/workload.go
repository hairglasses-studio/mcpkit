package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hairglasses-studio/mcpkit/registry"
)

// WorkloadIdentityProvider resolves service identity from the runtime environment.
type WorkloadIdentityProvider interface {
	// GetToken returns a bearer token for the current workload identity.
	GetToken(ctx context.Context) (string, error)
	// Name returns the provider name (e.g., "gcp", "aws").
	Name() string
}

// GCPMetadataProvider fetches identity tokens from the GCP metadata server.
type GCPMetadataProvider struct {
	Audience   string
	HTTPClient *http.Client
}

// NewGCPMetadataProvider creates a GCP workload identity provider.
func NewGCPMetadataProvider(audience string) *GCPMetadataProvider {
	return &GCPMetadataProvider{
		Audience:   audience,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *GCPMetadataProvider) Name() string { return "gcp" }

func (p *GCPMetadataProvider) GetToken(ctx context.Context) (string, error) {
	url := fmt.Sprintf("http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=%s", p.Audience)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("gcp metadata request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gcp metadata fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gcp metadata returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gcp metadata read: %w", err)
	}

	return string(body), nil
}

// AWSIMDSProvider fetches credentials from the AWS Instance Metadata Service (IMDSv2).
type AWSIMDSProvider struct {
	RoleSessionName string
	HTTPClient      *http.Client
}

// NewAWSIMDSProvider creates an AWS workload identity provider.
func NewAWSIMDSProvider() *AWSIMDSProvider {
	return &AWSIMDSProvider{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *AWSIMDSProvider) Name() string { return "aws" }

func (p *AWSIMDSProvider) GetToken(ctx context.Context) (string, error) {
	// Step 1: Get IMDSv2 session token
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://169.254.169.254/latest/api/token", nil)
	if err != nil {
		return "", fmt.Errorf("aws imds token request: %w", err)
	}
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")

	tokenResp, err := p.HTTPClient.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("aws imds token fetch: %w", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("aws imds token returned %d", tokenResp.StatusCode)
	}

	imdsToken, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return "", fmt.Errorf("aws imds token read: %w", err)
	}

	// Step 2: Get IAM role name
	roleReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://169.254.169.254/latest/meta-data/iam/security-credentials/", nil)
	if err != nil {
		return "", fmt.Errorf("aws imds role request: %w", err)
	}
	roleReq.Header.Set("X-aws-ec2-metadata-token", string(imdsToken))

	roleResp, err := p.HTTPClient.Do(roleReq)
	if err != nil {
		return "", fmt.Errorf("aws imds role fetch: %w", err)
	}
	defer roleResp.Body.Close()

	if roleResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("aws imds role returned %d", roleResp.StatusCode)
	}

	roleBytes, err := io.ReadAll(roleResp.Body)
	if err != nil {
		return "", fmt.Errorf("aws imds role read: %w", err)
	}
	roleName := string(roleBytes)

	// Step 3: Get credentials for the role
	credURL := fmt.Sprintf("http://169.254.169.254/latest/meta-data/iam/security-credentials/%s", roleName)
	credReq, err := http.NewRequestWithContext(ctx, http.MethodGet, credURL, nil)
	if err != nil {
		return "", fmt.Errorf("aws imds cred request: %w", err)
	}
	credReq.Header.Set("X-aws-ec2-metadata-token", string(imdsToken))

	credResp, err := p.HTTPClient.Do(credReq)
	if err != nil {
		return "", fmt.Errorf("aws imds cred fetch: %w", err)
	}
	defer credResp.Body.Close()

	if credResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("aws imds cred returned %d", credResp.StatusCode)
	}

	var creds struct {
		AccessKeyId     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		Token           string `json:"Token"`
	}
	if err := json.NewDecoder(credResp.Body).Decode(&creds); err != nil {
		return "", fmt.Errorf("aws imds cred decode: %w", err)
	}

	return creds.Token, nil
}

// AutoDetect probes the runtime environment and returns the first working provider.
// It tries GCP metadata first, then AWS IMDS.
func AutoDetect(ctx context.Context) (WorkloadIdentityProvider, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Try GCP
	gcp := NewGCPMetadataProvider("autodetect")
	gcpReq, err := http.NewRequestWithContext(probeCtx, http.MethodGet, "http://metadata.google.internal/", nil)
	if err == nil {
		gcpReq.Header.Set("Metadata-Flavor", "Google")
		if resp, err := gcp.HTTPClient.Do(gcpReq); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return gcp, nil
			}
		}
	}

	// Try AWS IMDSv2
	aws := NewAWSIMDSProvider()
	awsReq, err := http.NewRequestWithContext(probeCtx, http.MethodPut, "http://169.254.169.254/latest/api/token", nil)
	if err == nil {
		awsReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")
		if resp, err := aws.HTTPClient.Do(awsReq); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return aws, nil
			}
		}
	}

	return nil, fmt.Errorf("no workload identity provider detected")
}

type workloadIdentityKey struct{}

// WithWorkloadToken returns a context with the workload identity token.
func WithWorkloadToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, workloadIdentityKey{}, token)
}

// WorkloadToken returns the workload identity token from the context.
func WorkloadToken(ctx context.Context) string {
	s, _ := ctx.Value(workloadIdentityKey{}).(string)
	return s
}

// WorkloadMiddleware returns registry.Middleware that fetches the workload identity token
// and injects it into the context for downstream handlers.
func WorkloadMiddleware(provider WorkloadIdentityProvider) registry.Middleware {
	return func(name string, td registry.ToolDefinition, next registry.ToolHandlerFunc) registry.ToolHandlerFunc {
		return func(ctx context.Context, request registry.CallToolRequest) (*registry.CallToolResult, error) {
			token, err := provider.GetToken(ctx)
			if err != nil {
				return registry.MakeErrorResult(fmt.Sprintf("workload identity (%s): %v", provider.Name(), err)), nil
			}
			ctx = WithWorkloadToken(ctx, token)
			ctx = WithSubject(ctx, fmt.Sprintf("workload:%s", provider.Name()))
			return next(ctx, request)
		}
	}
}
