package api

import "github.com/gofiber/fiber/v2"

func (server *Server) Route(route fiber.Router) {
	route.Get("/api/trip/:tripId", server.GetTrip)
	route.Post("/api/trip/create", server.CreateTrip)
}
