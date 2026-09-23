package api

import "github.com/jackc/pgx/v5/pgxpool"

type Server struct {
	DB *pgxpool.Pool
}

func NewServer(db *pgxpool.Pool) *Server {
	return &Server{
		DB: db,
	}
}
