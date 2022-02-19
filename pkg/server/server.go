package server

import (
	"fmt"
	"github.com/kubespace/pipeline-plugin/pkg/router"
	"net/http"
)

type Server struct {
	*Config
	router *router.Router
}

func NewServer(config *Config) (*Server, error) {
	routerConfig := &router.Config{}
	r, err := router.NewRouter(routerConfig)
	if err != nil {
		return nil, err
	}
	return &Server{Config: config, router: r}, nil
}

func (s *Server) Run() error {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.Config.Port),
		Handler: s.router,
	}
	return server.ListenAndServe()
}
