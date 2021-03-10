/*
Copyright 2021
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhookrouter

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gw-tester/nse-injector-webhook/internal/core/domain"
	commonhdl "github.com/gw-tester/nse-injector-webhook/internal/handlers/commonhld"
	handler "github.com/gw-tester/nse-injector-webhook/internal/handlers/webhookhld"
	log "github.com/sirupsen/logrus"
)

type router struct {
	server        *http.Server
	sidecarConfig *domain.Config
}

// Router provides a server to process requests.
type Router interface {
	ListenAndServe()
	Close()
}

// New initialize a router object with user and control plane connections.
func New(server *http.Server, sc *domain.Config) Router {
	router := &router{
		sidecarConfig: sc,
		server:        server,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", commonhdl.NewLogger(handler.New(sc)).Do)
	router.server.Handler = mux

	return router
}

// ListenAndServe initiates user and control plane connections and waits for incomming requests.
func (r *router) ListenAndServe() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := r.server.ListenAndServeTLS("", ""); err != nil {
			log.WithError(err).Fatal("Failed to listen and serve webhook server")
		}
	}()
	log.Info("NSE webhook injector has started")

	<-sigCh
}

// Close removes rules and routes added by the Router and closes user plane connnection.
func (r *router) Close() {
	log.Info("Got OS shutdown signal, shutting down NSE webhook injector gracefully...")

	if err := r.server.Shutdown(context.Background()); err != nil {
		log.WithError(err).Error("Failed to shutting down the webhook server")
	}
}
