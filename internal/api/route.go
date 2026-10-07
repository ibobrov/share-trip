package api

import (
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) Route(route fiber.Router) {
	route.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{})))
	route.Get("/api/trip/:tripId", s.GetTrip)
	route.Post("/api/trip/create", s.CreateTrip)
	route.Post("/api/trip/publish", s.PublishTrip)
}
