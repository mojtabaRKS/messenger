package api

import (
	"arvan/message-gateway/intrernal/config"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Server struct {
	engine *gin.Engine
}

func New(appEnv config.AppEnv) *Server {
	if appEnv == config.ProductionEnv {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.RedirectTrailingSlash = false

	return &Server{
		engine: r,
	}
}

func (s *Server) Serve(ctx context.Context, address string) error {
	srv := &http.Server{
		Addr:    address,
		Handler: s.engine,
	}

	log.Info(fmt.Sprintf("rest server starting at: %s", address))
	srvError := make(chan error)
	go func() {
		srvError <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		// graceful shutdown
		log.Info("rest server is shutting down")
		return srv.Shutdown(ctx)
	case err := <-srvError:
		return err
	}
}
