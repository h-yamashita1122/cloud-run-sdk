package http

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/ishii1648/cloud-run-sdk/logging/zerolog"
)

type Error struct {
	// error message for cloud run administator
	Error error
	// error message for client user
	Message string
	// http status code for client user
	Code int
}

// It's usually a mistake to pass back the concrete type of an error rather than error,
// because it can make it difficult to catch errors,
// but it's the right thing to do here because ServeHTTP is the only place that sees the value and uses its contents.
type AppHandler func(http.ResponseWriter, *http.Request) *Error

func (fn AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		logger := zerolog.Ctx(r.Context())
		logger.Errorf("error : %v", err)
		http.Error(w, err.Message, err.Code)
	}
}

type Server struct {
	addr        string
	logger      *zerolog.Logger
	middlewares []Middleware
}

func NewServer(rootLogger *zerolog.Logger, projectID string, middlewares ...Middleware) *Server {
	port, isSet := os.LookupEnv("PORT")
	if !isSet {
		port = "8080"
	}

	hostAddr, isSet := os.LookupEnv("HOST_ADDR")
	if !isSet {
		hostAddr = "0.0.0.0"
	}

	middlewares = append([]Middleware{InjectLogger(rootLogger, projectID)}, middlewares...)

	return &Server{
		addr:        hostAddr + ":" + port,
		logger:      rootLogger,
		middlewares: middlewares,
	}
}

func (s *Server) Start(path string, handler AppHandler, stopCh <-chan struct{}) {
	mux := http.NewServeMux()
	mux.Handle(path, Chain(handler, s.middlewares...))

	server := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Errorf("server closed with error : %v", err)
		}
	}()

	<-stopCh

	s.logger.Info("recive SIGTERM or SIGINT")

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	if err := server.Shutdown(ctx); err != nil {
		s.logger.Errorf("failed to shutdown HTTP Server : %v", err)
	}

	s.logger.Info("HTTP Server shutdowned")
}
