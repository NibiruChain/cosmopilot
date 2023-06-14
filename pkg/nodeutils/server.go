package nodeutils

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
)

type Server struct {
	router *mux.Router
	cfg    *Options
	client *chainutils.QueryClient
}

func NewServer(opts ...Option) (*Server, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	client, err := chainutils.NewQueryClient("127.0.0.1:9090")
	if err != nil {
		return nil, err
	}

	return &Server{
		cfg:    options,
		router: mux.NewRouter(),
		client: client,
	}, nil
}

func (s *Server) StartServer() error {
	s.registerRoutes()
	log.Infof("server started listening on %s:%d ...\n\n", s.cfg.Host, s.cfg.Port)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port), s.router)
}

func (s *Server) registerRoutes() {
	s.router.HandleFunc("/ready", s.ready).Methods(http.MethodGet)
	s.router.HandleFunc("/health", s.health).Methods(http.MethodGet)
	s.router.HandleFunc("/data_size", s.dataSize).Methods(http.MethodGet)
}
