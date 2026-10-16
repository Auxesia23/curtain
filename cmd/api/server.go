package main

import (
	"net/http"
	"time"
)

type config struct {
	addr string
}

type server struct {
	cfg    config
	router http.Handler
}

func newServer(cfg config, router http.Handler) *server {
	return &server{cfg: cfg, router: router}
}

func (s *server) run() error {
	srv := http.Server{
		Addr:              s.cfg.addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}
