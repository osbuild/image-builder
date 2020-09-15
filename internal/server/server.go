//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types,client -o ../cloudapi/cloudapi_client.go ../cloudapi/cloudapi_client.yml
//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=server --generate types,chi-server,spec,client -o api.go api.yaml
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/osbuild/image-builder/internal/cloudapi"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi"
)

type Handlers struct{}

func (s *Handlers) GetVersion(w http.ResponseWriter, r *http.Request) {
	spec, err := GetSwagger()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	version := Version{spec.Info.Version}

	versionEncoded, err := json.Marshal(version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(versionEncoded)
}

func (s *Handlers) GetOpenapiJson(w http.ResponseWriter, r *http.Request) {
	spec, err := GetSwagger()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	spec.AddServer(&openapi3.Server{URL: RoutePrefix()})

	specEncoded, err := json.Marshal(spec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(specEncoded)
}

func (s *Handlers) GetDistributions(w http.ResponseWriter, r *http.Request) {
	distributions,err := AvailableDistributions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	distributionsEncoded, err := json.Marshal(distributions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(distributionsEncoded)
}

func (s *Handlers) GetArchitectures(w http.ResponseWriter, r *http.Request) {
	archs, err := ArchitecturesForImage(r.Context().Value("distribution").(string))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	archsEncoded, err := json.Marshal(archs)
	if err != nil {
	http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(archsEncoded)
}

func (s *Handlers) GetComposeStatus(w http.ResponseWriter, r *http.Request) {
	socket, ok := os.LookupEnv("OSBUILD_SERVICE")
	if !ok {
		socket = "http://127.0.0.1:80/"
	}
	endpoint := "compose/" + r.Context().Value("composeId").(string)

	resp, err := http.Get(socket + endpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var composeStatus ComposeStatus
	json.NewDecoder(resp.Body).Decode(&composeStatus)
	encoded, err := json.Marshal(composeStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded)
}

func (s *Handlers) ComposeImage(w http.ResponseWriter, r *http.Request) {
	socket, ok := os.LookupEnv("OSBUILD_SERVICE")
	if !ok {
		socket = "http://127.0.0.1:80/"
	}

	decoder := json.NewDecoder(r.Body)
	var composeRequest cloudapi.ComposeJSONRequestBody
	err := decoder.Decode(&composeRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed decoding the compose request %s", err.Error()), http.StatusBadRequest)
		return
	}

	if len(composeRequest.ImageRequests) != 1 {
		http.Error(w, "Exactly one image request should be included", http.StatusBadRequest)
		return
	}

	if len(composeRequest.ImageRequests[0].Repositories) != 0 {
		http.Error(w, "Repositories are specified by image-builder itself", http.StatusBadRequest)
		return
	}

	repositories, err := RepositoriesForImage(composeRequest.Distribution, composeRequest.ImageRequests[0].Architecture)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to retrieve repositories for image %s", err), http.StatusInternalServerError)
		return
	}
	composeRequest.ImageRequests[0].Repositories = repositories

	client, err := cloudapi.NewClient(socket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed constructing http client %s", err.Error()), http.StatusInternalServerError)
		return
	}

	resp, err := client.Compose(context.Background(), composeRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed posting compose request to osbuild-composer %s", err.Error()), http.StatusInternalServerError)
		return
	}
	if resp.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed posting compose request to osbuild-composer", resp.StatusCode)
			return
		}
		http.Error(w, fmt.Sprintf("Failed posting compose request to osbuild-composer: %s", body), resp.StatusCode)
		return
	}

	var composeResponse ComposeResponse
	json.NewDecoder(resp.Body).Decode(&composeResponse)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(composeResponse)
	if err != nil {
		panic("Failed to write response")
	}
}

func RoutePrefix() string {
	pathPrefix, ok := os.LookupEnv("PATH_PREFIX")
	if !ok {
		pathPrefix = "api"
	}
	appName, ok := os.LookupEnv("APP_NAME")
	if !ok {
		appName = "image-builder"
	}
	spec, err := GetSwagger()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("/%s/%s/v%s", pathPrefix, appName, spec.Info.Version)
}

func Run(address string) {
	fmt.Printf("🚀 Starting image-builder server on %s ...\n", address)
	var s Handlers
	router := chi.NewRouter()
	router.Route(RoutePrefix(), func(r chi.Router) {
		HandlerFromMux(&s, r)
	})
	http.ListenAndServe(address, router)
}
