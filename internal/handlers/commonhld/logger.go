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

package commonhdl

import (
	"net/http"

	"github.com/gw-tester/nse-injector-webhook/internal/handlers"
	log "github.com/sirupsen/logrus"
)

type logger struct {
	handler handlers.Handler
}

// Do handles the request by passing it to the real handler and logging the request details.
func (l logger) Do(writer http.ResponseWriter, request *http.Request) {
	log.WithFields(log.Fields{
		"request": request,
	}).Debug("Request received")

	l.handler.Do(writer, request)
}

// NewLogger constructs a new Logger middleware handler.
func NewLogger(h handlers.Handler) handlers.Handler {
	return logger{
		handler: h,
	}
}
