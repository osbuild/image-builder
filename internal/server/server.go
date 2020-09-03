//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=server --generate types,chi-server,spec -o api.go api.yaml
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

func (s *Handlers) GetComposeStatus(w http.ResponseWriter, r *http.Request) {
	socket, ok := os.LookupEnv("OSBUILD_SERVICE")
	if !ok {
		socket = "http://127.0.0.1:80/"
	}
	endpoint := "v1/compose/" + r.Context().Value("composeId").(string)
	log.Println(r.Context().Value("composeId").(string))

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
	endpoint := "v1/compose"

	var composeRequest ComposeRequest
	err := json.NewDecoder(r.Body).Decode(&composeRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Println("Starting compose")
	composeRequestEncoded, err := json.Marshal(composeRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.Post(socket+endpoint, "application/json", bytes.NewBuffer(composeRequestEncoded))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Finished compose post, writing response")

	var composeResponse ComposeResponse
	json.NewDecoder(resp.Body).Decode(&composeResponse)
	encoded, err := json.Marshal(composeResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(encoded)
}

func RoutePrefix() string {
	pathPrefix, ok := os.LookupEnv("PATH_PREFIX")
	if !ok {
		pathPrefix = "api"
	}
	appName, ok := os.LookupEnv("APP_NAME")
	if !ok {
		appName = "osbuild-installer"
	}
	spec, err := GetSwagger()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("/%s/%s/v%s", pathPrefix, appName, spec.Info.Version)
}

func Run(address string) {
	fmt.Printf("🚀 Starting osbuild-installer server on %s ...\n", address)
	var s Handlers
	router := chi.NewRouter()
	router.Route(RoutePrefix(), func(r chi.Router) {
		HandlerFromMux(&s, r)
	})
	http.ListenAndServe(address, router)
}
