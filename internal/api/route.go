package api

import (
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) Route(route fiber.Router, keycloakClientID string, requestTimeout time.Duration) {
	route.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})))

	trips := route.Group(
		"/api/trip",
		middleware.RequireClientRole(keycloakClientID, "client"),
	)

	trips.Get("/:tripId", middleware.TimeoutMiddleware(requestTimeout), s.GetTrip)
	trips.Post("/create", middleware.TimeoutMiddleware(requestTimeout), s.CreateTrip)
	trips.Post("/publish", middleware.TimeoutMiddleware(requestTimeout), s.PublishTrip)
}
