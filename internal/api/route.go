package api

import "github.com/gofiber/fiber/v2"

func (s *Server) Route(route fiber.Router) {
	route.Get("/api/trip/:tripId", s.GetTrip)
	route.Post("/api/trip/create", s.CreateTrip)
	route.Post("/api/trip/publish", s.PublishTrip)
}
