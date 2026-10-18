package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	RefreshTokenHeader = "X-Refresh-Token"
	KeycloakClaimsKey  = "keycloak_claims"
)

type KeycloakConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
}

type KeycloakClaims struct {
	Subject           string `json:"sub"`
	CompanyID         string `json:"company_id"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	AuthorizedParty   string `json:"azp"`
	ResourceAccess    map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

type keycloakTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func refreshAccessToken(
	ctx context.Context,
	client *http.Client,
	cfg KeycloakConfig,
	refreshToken string,
) (*keycloakTokenResponse, error) {
	if cfg.Issuer == "" {
		return nil, errors.New("keycloak issuer is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("keycloak client id is required")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.ClientID)
	form.Set("refresh_token", refreshToken)

	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}

	endpoint := strings.TrimRight(cfg.Issuer, "/") + "/protocol/openid-connect/token"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewBufferString(form.Encode()),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("keycloak#resp.Body.Close(): %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("keycloak token endpoint returned status %d", resp.StatusCode)
	}

	var token keycloakTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	if token.AccessToken == "" {
		return nil, errors.New("keycloak response does not contain access_token")
	}

	return &token, nil
}

func parseAccessTokenClaims(accessToken string) (*KeycloakClaims, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt must contain three parts")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims KeycloakClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	if claims.Subject == "" {
		return nil, errors.New("jwt does not contain sub claim")
	}

	return &claims, nil
}

func (c KeycloakClaims) HasClientRole(clientID string, role string) bool {
	access, ok := c.ResourceAccess[clientID]
	if !ok {
		return false
	}

	for _, current := range access.Roles {
		if current == role {
			return true
		}
	}

	return false
}

func RequireClientRole(clientID string, role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := ClaimsFromContext(c)
		if err != nil {
			return err
		}

		if !claims.HasClientRole(clientID, role) {
			return fiber.NewError(fiber.StatusForbidden, "access denied")
		}

		return c.Next()
	}
}

func KeycloakRefreshTokenMiddleware(cfg KeycloakConfig) fiber.Handler {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	return func(c *fiber.Ctx) error {
		refreshToken := c.Get(RefreshTokenHeader)
		if refreshToken == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing refresh token")
		}

		token, err := refreshAccessToken(c.UserContext(), client, cfg, refreshToken)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid refresh token")
		}

		claims, err := parseAccessTokenClaims(token.AccessToken)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid access token")
		}

		c.Locals(KeycloakClaimsKey, claims)

		return c.Next()
	}
}

func ClaimsFromContext(c *fiber.Ctx) (*KeycloakClaims, error) {
	value := c.Locals(KeycloakClaimsKey)

	claims, ok := value.(*KeycloakClaims)
	if !ok {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "missing token claims")
	}

	return claims, nil
}
