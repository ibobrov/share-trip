package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := s.DB.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("NOT READY")
	}

	return c.Status(fiber.StatusOK).SendString("OK")
}
