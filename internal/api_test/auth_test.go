package api_test

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
)

const (
	testKeycloakClientID = "sharetrip-api"
	testUserHeader       = "X-Test-User-ID"
	testRoleHeader       = "X-Test-Role"
)

// Используется только в тестовом приложении.
func testAuthMiddleware(c *fiber.Ctx) error {
	subject := c.Get(testUserHeader)
	if subject == "" {
		return fiber.NewError(
			fiber.StatusUnauthorized,
			"missing test user",
		)
	}

	roles := []string{}
	if role := c.Get(testRoleHeader); role != "" {
		roles = append(roles, role)
	}

	claims := &middleware.KeycloakClaims{
		Subject: subject,
		ResourceAccess: map[string]struct {
			Roles []string `json:"roles"`
		}{
			testKeycloakClientID: {Roles: roles},
		},
	}

	c.Locals(middleware.KeycloakClaimsKey, claims)

	return c.Next()
}

type requestOption func(*http.Request)

func withUser(clientID uuid.UUID) requestOption {
	return func(req *http.Request) {
		req.Header.Set(testUserHeader, clientID.String())
		req.Header.Set(testRoleHeader, "client")
	}
}
