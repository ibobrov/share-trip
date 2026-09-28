package api

import (
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB          *pgxpool.Pool
	tripService *service.TripService
}

func NewServer(db *pgxpool.Pool, tripService *service.TripService) *Server {
	return &Server{
		DB:          db,
		tripService: tripService,
	}
}
