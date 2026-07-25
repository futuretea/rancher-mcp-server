package mcp

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"

	"github.com/futuretea/rancher-mcp-server/pkg/core/config"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
)

const (
	oauthSigningMethod = "RS256"
	oauthMetadataPath  = "/.well-known/oauth-protected-resource"
)

var (
	oauthJWKSConstructionTimeout = 10 * time.Second
	oauthJWKSRefreshInterval     = time.Hour
	oauthJWKSHTTPClient          = &http.Client{}
	errInvalidOAuthAuthorization = errors.New("invalid OAuth Bearer token")
)

type oauthTokenContextKey struct{}

type oauthTokenVerifier struct {
	keyfunc       keyfunc.Keyfunc
	issuer        string
	cancelRefresh context.CancelFunc
}

func newOAuthTokenVerifier(staticConfig *config.StaticConfig) (*oauthTokenVerifier, error) {
	refreshContext, cancelRefresh := context.WithCancel(context.Background())
	storage, err := newOAuthJWKSStorage(refreshContext, staticConfig.RancherOAuthJWKSURL, oauthJWKSClient(staticConfig.RancherTLSInsecure))
	if err != nil {
		cancelRefresh()
		return nil, fmt.Errorf("parse OAuth JWKS: %w", err)
	}
	jwtKeyfunc, err := keyfunc.New(keyfunc.Options{Ctx: refreshContext, Storage: storage})
	if err != nil {
		cancelRefresh()
		return nil, fmt.Errorf("create OAuth keyfunc: %w", err)
	}
	if err := validateOAuthJWKS(jwtKeyfunc); err != nil {
		cancelRefresh()
		return nil, err
	}

	return &oauthTokenVerifier{
		keyfunc:       jwtKeyfunc,
		issuer:        strings.TrimSuffix(staticConfig.RancherOAuthAuthorizationServerURL, "/authorize"),
		cancelRefresh: cancelRefresh,
	}, nil
}

func newOAuthJWKSStorage(ctx context.Context, url string, client *http.Client) (jwkset.Storage, error) {
	remoteStorage, err := jwkset.NewStorageFromHTTP(url, jwkset.HTTPClientStorageOptions{
		Client:              client,
		Ctx:                 ctx,
		HTTPTimeout:         oauthJWKSConstructionTimeout,
		RefreshErrorHandler: oauthJWKSRefreshErrorHandler,
		RefreshInterval:     oauthJWKSRefreshInterval,
		Storage:             oauthJWKSStorage{Storage: jwkset.NewMemoryStorage()},
	})
	if err != nil {
		return nil, fmt.Errorf("create OAuth JWKS storage: %w", err)
	}

	storage, err := jwkset.NewHTTPClient(jwkset.HTTPClientOptions{
		HTTPURLs:          map[string]jwkset.Storage{url: remoteStorage},
		RateLimitWaitMax:  time.Minute,
		RefreshUnknownKID: rate.NewLimiter(rate.Every(5*time.Minute), 1),
	})
	if err != nil {
		return nil, fmt.Errorf("create OAuth JWKS client: %w", err)
	}
	return storage, nil
}

func oauthJWKSRefreshErrorHandler(context.Context, error) {
	logging.Warn("OAuth JWKS refresh failed; retaining the last known good keyset")
}

type oauthJWKSStorage struct {
	jwkset.Storage
}

func (s oauthJWKSStorage) KeyReplaceAll(ctx context.Context, keys []jwkset.JWK) error {
	verificationKeys := oauthVerificationKeys(keys)
	if len(verificationKeys) == 0 {
		return errors.New("OAuth JWKS has no RS256 verification key")
	}
	return s.Storage.KeyReplaceAll(ctx, verificationKeys)
}

// Close stops the background JWKS refresh loop.
func (v *oauthTokenVerifier) Close() {
	if v != nil && v.cancelRefresh != nil {
		v.cancelRefresh()
	}
}

func oauthJWKSClient(insecure bool) *http.Client {
	if !insecure {
		return oauthJWKSHTTPClient
	}

	client := *oauthJWKSHTTPClient
	transport, ok := client.Transport.(*http.Transport)
	if client.Transport == nil {
		transport = http.DefaultTransport.(*http.Transport)
		ok = true
	}
	if !ok {
		return &client
	}

	transport = transport.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	// #nosec G402 -- rancher_tls_insecure is an explicit existing configuration choice.
	transport.TLSClientConfig.InsecureSkipVerify = true
	client.Transport = transport
	return &client
}

func validateOAuthJWKS(jwtKeyfunc keyfunc.Keyfunc) error {
	keys, err := jwtKeyfunc.Storage().KeyReadAll(context.Background())
	if err != nil {
		return fmt.Errorf("inspect OAuth JWKS: %w", err)
	}
	return validateOAuthJWKs(keys)
}

func validateOAuthJWKs(keys []jwkset.JWK) error {
	if len(oauthVerificationKeys(keys)) != 0 {
		return nil
	}
	return errors.New("OAuth JWKS has no RS256 verification key")
}

func oauthVerificationKeys(keys []jwkset.JWK) []jwkset.JWK {
	verificationKeys := make([]jwkset.JWK, 0, len(keys))
	for _, key := range keys {
		algorithm := key.Marshal().ALG.String()
		if algorithm != "" && algorithm != oauthSigningMethod {
			continue
		}
		if key.Marshal().USE != "" && key.Marshal().USE != jwkset.UseSig {
			continue
		}
		if keyOperations := key.Marshal().KEYOPS; len(keyOperations) > 0 && !containsVerificationKeyOperation(keyOperations) {
			continue
		}
		if _, ok := key.Key().(*rsa.PublicKey); ok {
			verificationKeys = append(verificationKeys, key)
		}
	}
	return verificationKeys
}

func containsVerificationKeyOperation(keyOperations []jwkset.KEYOPS) bool {
	for _, keyOperation := range keyOperations {
		if keyOperation == jwkset.KeyOpsVerify {
			return true
		}
	}
	return false
}

func (v *oauthTokenVerifier) verifyAuthorization(authorization string) (string, error) {
	fields := strings.Fields(authorization)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", errInvalidOAuthAuthorization
	}

	token, err := jwt.Parse(
		fields[1],
		v.keyfunc.Keyfunc,
		jwt.WithValidMethods([]string{oauthSigningMethod}),
		jwt.WithIssuer(v.issuer),
		jwt.WithLeeway(10*time.Second),
	)
	if err != nil || !token.Valid {
		return "", errInvalidOAuthAuthorization
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !hasRequiredOAuthScopes(claims) {
		return "", errInvalidOAuthAuthorization
	}

	return fields[1], nil
}

func hasRequiredOAuthScopes(claims jwt.MapClaims) bool {
	rawScopes, ok := claims["scope"].([]any)
	if !ok {
		return false
	}

	scopes := make(map[string]struct{}, len(rawScopes))
	for _, rawScope := range rawScopes {
		scope, ok := rawScope.(string)
		if !ok {
			return false
		}
		scopes[scope] = struct{}{}
	}

	_, hasOfflineAccess := scopes["offline_access"]
	_, hasRancherMCP := scopes["rancher:mcp"]
	return hasOfflineAccess && hasRancherMCP
}

func oauthTokenContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, oauthTokenContextKey{}, token)
}

func verifiedOAuthTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(oauthTokenContextKey{}).(string)
	return token, ok && token != ""
}

func (v *oauthTokenVerifier) middleware(resourceURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := v.verifyAuthorization(r.Header.Get("Authorization"))
		if err != nil {
			writeOAuthUnauthorized(w, resourceURL)
			return
		}
		next.ServeHTTP(w, r.WithContext(oauthTokenContext(r.Context(), token)))
	})
}

func writeOAuthUnauthorized(w http.ResponseWriter, resourceURL string) {
	metadataURL := strings.TrimRight(resourceURL, "/") + oauthMetadataPath
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=%q", metadataURL))
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

type oauthProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

func oauthProtectedResourceMetadataHandler(staticConfig *config.StaticConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(oauthProtectedResourceMetadata{
			Resource:             staticConfig.RancherOAuthResourceURL,
			AuthorizationServers: []string{staticConfig.RancherOAuthAuthorizationServerURL},
			ScopesSupported:      []string{"offline_access", "rancher:mcp"},
		})
	})
}
