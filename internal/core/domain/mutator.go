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

package domain

import (
	"encoding/json"
	"strings"

	log "github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	annotationEndpointsKey        = "ns.networkservicemesh.io/endpoints"
	annotationStatusKey           = "ns.networkservicemesh.io/status"
	annotationStatusInjectedValue = "injected"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type podMutator struct {
	admisssionReview *admissionv1.AdmissionReview
	sidecar          *Config
}

// Mutator injects NSE sidecar.
type Mutator interface {
	Mutate() *admissionv1.AdmissionResponse
}

// New creates a mutator instance.
func New(ar *admissionv1.AdmissionReview, s *Config) Mutator {
	return &podMutator{
		admisssionReview: ar,
		sidecar:          s,
	}
}

// Check whether the target resoured need to be mutated.
func isRequired(metadata *metav1.ObjectMeta) bool {
	switch metadata.Namespace {
	case
		metav1.NamespaceSystem,
		metav1.NamespacePublic:
		log.Info("Skip mutation, it's in special namespace")

		return false
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	log.WithFields(log.Fields{
		"annotations": annotations,
	}).Debug("Pod's annotations")

	// Note: if status key is diffent than injected and it has NSM endpoint definition
	return strings.ToLower(annotations[annotationStatusKey]) != annotationStatusInjectedValue &&
		annotations[annotationEndpointsKey] != ""
}

func getPatchOperation(r interface{}) patchOperation {
	const (
		containersPath = "/spec/containers"
		volumesPath    = "/spec/volumes"
	)

	result := patchOperation{
		Op: "add",
	}

	switch resource := r.(type) {
	case corev1.Container:
		result.Value = []corev1.Container{resource}
		result.Path = containersPath
	case corev1.Volume:
		result.Value = []corev1.Volume{resource}
		result.Path = volumesPath
	}

	return result
}

func (m *podMutator) createPatch(pod corev1.Pod) (patchOp []patchOperation) {
	isFirstContainer := len(pod.Spec.Containers) == 0

	for _, container := range m.sidecar.Containers {
		patch := getPatchOperation(container)

		if isFirstContainer {
			isFirstContainer = false
		} else {
			patch.Path += "/-"
			patch.Value = container
		}

		patchOp = append(patchOp, patch)

		log.WithFields(log.Fields{
			"patch": patch,
		}).Debug("Container patch added")
	}

	isFirstVolumes := len(pod.Spec.Volumes) == 0

	for _, volume := range m.sidecar.Volumes {
		patch := getPatchOperation(volume)

		if isFirstVolumes {
			isFirstVolumes = false
		} else {
			patch.Path += "/-"
			patch.Value = volume
		}

		patchOp = append(patchOp, patch)

		log.WithFields(log.Fields{
			"patch": patch,
		}).Debug("Volume patch added")
	}

	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[annotationStatusKey] = annotationStatusInjectedValue

	patchOp = append(patchOp, patchOperation{
		Op:    "add",
		Path:  "/metadata/annotations",
		Value: annotations,
	})

	log.WithFields(log.Fields{
		"annotations": annotations,
	}).Debug("Annotations patch added")

	return patchOp
}

func (m *podMutator) Mutate() *admissionv1.AdmissionResponse {
	var pod corev1.Pod

	req := m.admisssionReview.Request

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.WithError(err).Warn("could not unmarshal raw object")

		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.WithFields(log.Fields{
		"Kind":           req.Kind,
		"Namespace":      req.Namespace,
		"Name":           req.Name,
		"Pod Name":       pod.Name,
		"UID":            req.UID,
		"patchOperation": req.Operation,
		"UserInfo":       req.UserInfo,
	}).Info("Admission Review received")

	// determine whether to perform mutation
	if !isRequired(&pod.ObjectMeta) {
		log.Info("Skipping mutation due to policy check")

		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	response, err := json.Marshal(m.createPatch(pod))
	if err != nil {
		log.WithError(err).Warn("could not encode the patch operations of pod")

		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   response,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch

			return &pt
		}(),
	}
}
