package apis

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-http-utils/headers"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-k8s-controllers/api/payloads"
	"code.cloudfoundry.org/cf-k8s-controllers/api/repositories"
)

const (
	SpaceManifestApplyEndpoint = "/v3/spaces/{spaceGUID}/actions/apply_manifest"
	SpaceManifestDiffEndpoint  = "/v3/spaces/{spaceGUID}/manifest_diff"
)

type SpaceManifestHandler struct {
	logger              logr.Logger
	serverURL           url.URL
	applyManifestAction ApplyManifestAction
	spaceRepo           repositories.CFSpaceRepository
	buildClient         ClientBuilder
	k8sConfig           *rest.Config // TODO: this would be global for all requests, not what we want
}

//counterfeiter:generate -o fake -fake-name ApplyManifestAction . ApplyManifestAction
type ApplyManifestAction func(ctx context.Context, c client.Client, spaceGUID string, manifest payloads.Manifest) error

func NewSpaceManifestHandler(
	logger logr.Logger,
	serverURL url.URL,
	applyManifestAction ApplyManifestAction,
	spaceRepo repositories.CFSpaceRepository,
	buildClient ClientBuilder,
	k8sConfig *rest.Config) *SpaceManifestHandler {
	return &SpaceManifestHandler{
		logger:              logger,
		serverURL:           serverURL,
		applyManifestAction: applyManifestAction,
		spaceRepo:           spaceRepo,
		buildClient:         buildClient,
		k8sConfig:           k8sConfig,
	}
}

func (h *SpaceManifestHandler) RegisterRoutes(router *mux.Router) {
	router.Path(SpaceManifestApplyEndpoint).Methods("POST").HandlerFunc(h.applyManifestHandler)
	router.Path(SpaceManifestDiffEndpoint).Methods("POST").HandlerFunc(h.validateSpaceVisible(h.diffManifestHandler))
}

func (h *SpaceManifestHandler) applyManifestHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	spaceGUID := vars["spaceGUID"]

	var manifest payloads.Manifest
	rme := decodeAndValidateYAMLPayload(r, &manifest)
	if rme != nil {
		w.Header().Set("Content-Type", "application/json")
		writeRequestMalformedErrorResponse(w, rme)
		return
	}

	client, err := h.buildClient(h.k8sConfig, r.Header.Get(headers.Authorization))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		h.logger.Error(err, "Unable to create Kubernetes client")
		writeUnknownErrorResponse(w)
		return
	}

	err = h.applyManifestAction(r.Context(), client, spaceGUID, manifest)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		h.logger.Error(err, "error applying the manifest")
		writeUnknownErrorResponse(w)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("%s/v3/jobs/sync-space.apply_manifest-%s", h.serverURL.String(), spaceGUID))
	w.WriteHeader(http.StatusAccepted)
}

func (h *SpaceManifestHandler) diffManifestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"diff":[]}`))
}

func decodeAndValidateYAMLPayload(r *http.Request, object interface{}) *requestMalformedError {
	decoder := yaml.NewDecoder(r.Body)
	defer r.Body.Close()
	decoder.KnownFields(false) // TODO: change this to true once we've added all manifest fields to payloads.Manifest
	err := decoder.Decode(object)
	if err != nil {
		Logger.Error(err, fmt.Sprintf("Unable to parse the YAML body: %T: %q", err, err.Error()))
		return &requestMalformedError{
			httpStatus:    http.StatusBadRequest,
			errorResponse: newMessageParseError(),
		}
	}

	return validatePayload(object)
}

func (h *SpaceManifestHandler) validateSpaceVisible(hf http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		spaceGUID := vars["spaceGUID"]
		w.Header().Set("Content-Type", "application/json")

		spaces, err := h.spaceRepo.FetchSpaces(r.Context(), []string{}, []string{})
		if err != nil {
			h.logger.Error(err, "Failed to list spaces")
			writeUnknownErrorResponse(w)
			return
		}

		spaceNotFound := true
		for _, space := range spaces {
			if space.GUID == spaceGUID {
				spaceNotFound = false
				break
			}
		}

		if spaceNotFound {
			writeNotFoundErrorResponse(w, "Space")
			return
		}

		hf.ServeHTTP(w, r)
	})
}
