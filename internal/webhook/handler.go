package webhook

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

const (
	InjectLabel     = "readiness-controller.io/inject"
	ConditionPrefix = "controller.rc/"
)

func init() {
	_ = admissionv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
}

func HandleMutate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	deserializer := codecs.UniversalDeserializer()
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		log.Printf("Error decoding admission review: %v", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	response := processAdmissionReview(ar)

	respAr := admissionv1.AdmissionReview{
		TypeMeta: ar.TypeMeta,
		Response: response,
	}
	if ar.Request != nil {
		respAr.Response.UID = ar.Request.UID
	}

	respBytes, err := json.Marshal(respAr)
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("Failed to write admission response: %v", err)
	}
}

func processAdmissionReview(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request
	if req == nil || req.Kind.Kind != "Deployment" {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	var deployment appsv1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
		return &admissionv1.AdmissionResponse{Allowed: true, Result: &metav1.Status{Message: err.Error()}}
	}

	gateName, ok := deployment.Labels[InjectLabel]
	if !ok {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	log.Printf("Injecting readiness gate '%s' into deployment %s", gateName, deployment.Name)

	fullConditionType := ConditionPrefix + gateName
	patch := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/template/spec/readinessGates",
			"value": []map[string]string{{"conditionType": fullConditionType}},
		},
	}

	if len(deployment.Spec.Template.Spec.ReadinessGates) > 0 {
		patch = []map[string]interface{}{
			{
				"op":    "add",
				"path":  "/spec/template/spec/readinessGates/-",
				"value": map[string]string{"conditionType": fullConditionType},
			},
		}
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return &admissionv1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}
