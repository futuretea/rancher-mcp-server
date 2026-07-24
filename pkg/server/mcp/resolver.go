package mcp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/futuretea/rancher-mcp-server/pkg/client/norman"
	"github.com/futuretea/rancher-mcp-server/pkg/client/steve"
	"github.com/futuretea/rancher-mcp-server/pkg/toolset"
)

// SteveFactory constructs a Steve client from URL, token and insecure flag.
type SteveFactory func(serverURL, token string, insecure bool) *steve.Client

// NormanFactory constructs a Norman client from URL, token and insecure flag.
type NormanFactory func(serverURL, token string, insecure bool) (*norman.Client, error)

// staticResolver always returns the pre-built static CombinedClient.
type staticResolver struct {
	client *toolset.CombinedClient
}

func (r *staticResolver) Resolve(_ context.Context) (*toolset.CombinedClient, error) {
	return r.client, nil
}

// requestTokenResolver builds a request-scoped CombinedClient from the Bearer token in ctx.
type requestTokenResolver struct {
	serverURL     string
	insecure      bool
	steveFactory  SteveFactory
	normanFactory NormanFactory
	metrics       Metrics
}

func (r *requestTokenResolver) Resolve(ctx context.Context) (*toolset.CombinedClient, error) {
	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordClientResolveDuration(time.Since(start))
		}
	}()

	token, err := requestTokenFromContext(ctx)
	if err != nil {
		if r.metrics != nil {
			r.metrics.IncrementRancherRequestErrors()
		}
		return nil, err
	}
	return resolveRequestScopedClient(r.serverURL, r.insecure, token, r.steveFactory, r.normanFactory, r.metrics)
}

// oauthTokenResolver builds a request-scoped CombinedClient from the verified OAuth token in ctx.
type oauthTokenResolver struct {
	serverURL     string
	insecure      bool
	steveFactory  SteveFactory
	normanFactory NormanFactory
	metrics       Metrics
}

func (r *oauthTokenResolver) Resolve(ctx context.Context) (*toolset.CombinedClient, error) {
	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordClientResolveDuration(time.Since(start))
		}
	}()

	token, ok := verifiedOAuthTokenFromContext(ctx)
	if !ok {
		if r.metrics != nil {
			r.metrics.IncrementRancherRequestErrors()
		}
		return nil, errors.New("missing verified OAuth token")
	}
	return resolveRequestScopedClient(r.serverURL, r.insecure, token, r.steveFactory, r.normanFactory, r.metrics)
}

func resolveRequestScopedClient(
	serverURL string,
	insecure bool,
	token string,
	steveFactory SteveFactory,
	normanFactory NormanFactory,
	metrics Metrics,
) (*toolset.CombinedClient, error) {

	steveClient := steveFactory(serverURL, token, insecure)

	// Create the Norman client but do not fail the whole request if it cannot be
	// built. This mirrors static-mode behavior, where a Norman startup failure is
	// logged but Kubernetes tools remain available. If a tool actually needs
	// Norman, ValidateNormanClient will report the configuration error.
	normanClient, err := normanFactory(serverURL, token, insecure)
	if err != nil {
		if metrics != nil {
			metrics.IncrementRancherRequestErrors()
		}
	}

	if metrics != nil {
		metrics.RecordClientResolveMemoryBytes(readMemoryBytes())
	}

	return toolset.NewCombinedClient(normanClient, steveClient, true), nil
}

func bearerTokenFromContext(ctx context.Context) (string, error) {
	raw := ctx.Value(authorizationKey)
	authHeader, ok := raw.(string)
	if !ok {
		return "", errors.New("missing Authorization header: per-request Rancher token is required")
	}

	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", errors.New("malformed Authorization header: expected Bearer token")
	}

	return fields[1], nil
}

func requestTokenFromContext(ctx context.Context) (string, error) {
	if ctx.Value(authorizationKey) != nil {
		return bearerTokenFromContext(ctx)
	}

	if token, ok := ctx.Value(rancherTokenKey).(string); ok && token != "" {
		return token, nil
	}

	return bearerTokenFromContext(ctx)
}
