package api

import (
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) Route(route fiber.Router, keycloakClientID string) {
	route.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})))

	trips := route.Group(
		"/api/trip",
		middleware.TimeoutMiddleware(2*time.Second),
		middleware.RequireClientRole(keycloakClientID, "client"),
	)

	trips.Get("/:tripId", s.GetTrip)
	trips.Post("/create", s.CreateTrip)
	trips.Post("/publish", s.PublishTrip)
}
