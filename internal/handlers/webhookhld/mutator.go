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

package webhookhld

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/gw-tester/nse-injector-webhook/internal/core/domain"
	"github.com/gw-tester/nse-injector-webhook/internal/handlers"
	log "github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// Handler stores the mutator sidecar configuration used to generate mutation responses.
type mutator struct {
	sidecarConfig *domain.Config
}

// New creates a Mutator handler instance.
func New(config *domain.Config) handlers.Handler {
	return mutator{
		sidecarConfig: config,
	}
}

func getAdmissionReview(request *http.Request) (*admissionv1.AdmissionReview, error) {
	ar := &admissionv1.AdmissionReview{}

	var body []byte

	if request.Body != nil {
		if data, err := ioutil.ReadAll(request.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		return ar, handlers.ErrEmptyBody
	}

	// verify the content type is accurate
	contentType := request.Header.Get("Content-Type")
	if contentType != "application/json" {
		log.Errorf("Content-Type=%s, expect application/json", contentType)

		return ar, handlers.ErrUnexpectedContentType
	}

	codecs := serializer.NewCodecFactory(runtime.NewScheme())
	deserializer := codecs.UniversalDeserializer()

	if _, _, err := deserializer.Decode(body, nil, ar); err != nil {
		log.Errorf("Can't decode body: %v", err)

		return ar, handlers.ErrDecodeBody
	}

	return ar, nil
}

// Do injects a sidecar to pods that have NSM endpoint annotations.
func (h mutator) Do(writer http.ResponseWriter, request *http.Request) {
	admissionReview := admissionv1.AdmissionReview{}

	ar, err := getAdmissionReview(request)
	if err != nil {
		switch {
		case errors.Is(err, handlers.ErrDecodeBody):
			admissionReview.Response = &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		case errors.Is(err, handlers.ErrUnexpectedContentType):
			http.Error(writer, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)

			return
		default:
			http.Error(writer, err.Error(), http.StatusBadRequest)

			return
		}
	} else {
		admissionReview.Response = domain.New(ar, h.sidecarConfig).Mutate()
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}

		log.WithFields(log.Fields{
			"Patch": string(admissionReview.Response.Patch),
			"ID":    ar.Request.UID,
		}).Info("Admission response added to the admission review")
	}

	response, err := json.Marshal(admissionReview)
	if err != nil {
		log.WithError(err).Warn("can't encode response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}

	if _, err := writer.Write(response); err != nil {
		log.WithError(err).Warn("can't write response")
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
