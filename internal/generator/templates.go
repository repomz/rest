package generator

const commonServerErrorTemplate = `package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"{{ .Module }}/internal/app/common/slugerrors"
)

func InternalError(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Internal server error", http.StatusInternalServerError)
}

func BadRequest(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Bad request", http.StatusBadRequest)
}

func NotFound(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Not found", http.StatusNotFound)
}

func RespondWithError(err error, w http.ResponseWriter, r *http.Request) {
	var slugError slugerrors.SlugError
	if !errors.As(err, &slugError) {
		InternalError("internal-server-error", err, w, r)
		return
	}

	switch slugError.ErrorType() {
	case slugerrors.ErrorTypeBadRequest:
		BadRequest(slugError.Slug(), slugError, w, r)
	case slugerrors.ErrorTypeNotFound:
		NotFound(slugError.Slug(), slugError, w, r)
	default:
		InternalError(slugError.Slug(), slugError, w, r)
	}
}

func httpRespondWithError(err error, slug string, w http.ResponseWriter, _ *http.Request, msg string, status int) {
	log.Printf("error: %s, slug: %s, msg: %s", err, slug, msg)

	resp := ErrorResponse{Slug: slug}
	if os.Getenv("DEBUG_ERRORS") != "" && err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

type ErrorResponse struct {
	Slug  string ` + "`json:\"slug\"`" + `
	Error string ` + "`json:\"error,omitempty\"`" + `
}
`

const commonServerOKTemplate = `package server

import (
	"encoding/json"
	"net/http"
)

func RespondOK(data interface{}, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}
`

const slugErrorsTemplate = `package slugerrors

type ErrorType struct {
	name string
}

var (
	ErrorTypeBadRequest = ErrorType{"bad-request"}
	ErrorTypeNotFound   = ErrorType{"not-found"}
	ErrorTypeInternal   = ErrorType{"internal"}
)

type SlugError struct {
	error     string
	slug      string
	errorType ErrorType
}

func (e SlugError) Error() string {
	return e.error
}

func (e SlugError) Slug() string {
	return e.slug
}

func (e SlugError) ErrorType() ErrorType {
	return e.errorType
}

func NewBadRequestError(error string, slug string) SlugError {
	return SlugError{error: error, slug: slug, errorType: ErrorTypeBadRequest}
}

func NewNotFoundError(error string, slug string) SlugError {
	return SlugError{error: error, slug: slug, errorType: ErrorTypeNotFound}
}
`

const configTemplate = `package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr string
	DB_DSN   string
}

func Read() Config {
	_ = godotenv.Load()

	cfg := Config{
		HTTPAddr: ":8080",
		DB_DSN:   os.Getenv("DB_DSN"),
	}
	if httpAddr := os.Getenv("HTTP_ADDR"); httpAddr != "" {
		cfg.HTTPAddr = httpAddr
	}
	return cfg
}
`

const domainErrorTemplate = `package domain

import "errors"

var (
	ErrRequired = errors.New("required value")
	ErrNotFound = errors.New("not found")
)
`

const domainModelTemplate = `package domain

import (
{{- if tableNeedsFmt .Table }}
	"fmt"
{{- end }}
{{- if hasImport .Table.Columns "time" }}
	"time"
{{- end }}
{{- if hasImport .Table.Columns "uuid" }}
	"github.com/google/uuid"
{{- end }}

	"{{ .DBImport }}"
)

type {{ .Table.GoName }} struct {
{{- range .Table.Columns }}
	{{ .GoName }} {{ .GoType }}
{{- end }}
}

type {{ .Table.GoName }}DB struct {
{{- range .Table.CreateCols }}
	{{ .GoName }} {{ .GoType }}
{{- end }}
}

type {{ .Table.GoName }}DBData struct {
{{- range .Table.CreateCols }}
	{{ .GoName }} {{ .GoType }}
{{- end }}
}

func New{{ .Table.GoName }}DB(data {{ .Table.GoName }}DBData) ({{ .Table.GoName }}DB, error) {
	item := {{ .Table.GoName }}DB{
{{- range .Table.CreateCols }}
		{{ .GoName }}: data.{{ .GoName }},
{{- end }}
	}
	if err := item.Validate(); err != nil {
		return {{ .Table.GoName }}DB{}, err
	}
	return item, nil
}

func (item {{ .Table.GoName }}DB) Validate() error {
{{- range .Table.CreateCols }}
{{- if .Required }}
	if !({{ .ValidCheck }}) {
		return fmt.Errorf("%w: {{ .JSONName }}", ErrRequired)
	}
{{- end }}
{{- end }}
	return nil
}

{{- range .Table.Endpoints }}
type {{ .Name }}Params struct {
{{- range .Params }}
	{{ .GoName }} {{ .GoType }}
{{- end }}
}

func (params {{ .Name }}Params) Validate() error {
{{- range .Params }}
{{- if .Required }}
	if !({{ .ValidCheck }}) {
		return fmt.Errorf("%w: {{ .JSONName }}", ErrRequired)
	}
{{- end }}
{{- end }}
	return nil
}

{{- end }}

func New{{ .Table.GoName }}FromDB(item db.{{ .Table.GoName }}) {{ .Table.GoName }} {
	return {{ .Table.GoName }}{
{{- range .Table.Columns }}
		{{ .GoName }}: {{ toDomainValue . }},
{{- end }}
	}
}
`

const serviceTemplate = `package services

import (
	"context"

	"github.com/google/uuid"
	"{{ .Module }}/internal/app/domain"
)

type {{ .Table.GoName }}Repository interface {
{{- if .Queries.GetAll }}
	GetAll{{ .Table.GoPlural }}(ctx context.Context) ([]domain.{{ .Table.GoName }}, error)
{{- end }}
{{- if .Queries.GetByID }}
	Get{{ .Table.GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .Table.GoName }}, error)
{{- end }}
{{- if .Queries.Create }}
	Create{{ .Table.GoName }}(ctx context.Context, item domain.{{ .Table.GoName }}DB) (domain.{{ .Table.GoName }}, error)
{{- end }}
{{- if .Queries.Delete }}
	Delete{{ .Table.GoName }}(ctx context.Context, id uuid.UUID) error
{{- end }}
{{- if .Queries.DeleteAll }}
	DeleteAll{{ .Table.GoPlural }}(ctx context.Context) error
{{- end }}
{{- range .Table.Endpoints }}
	{{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) ({{ if eq .Result "many" }}[]{{ end }}domain.{{ $.Table.GoName }}, error)
{{- end }}
}

type {{ .Table.GoName }}Service struct {
	repo {{ .Table.GoName }}Repository
}

func New{{ .Table.GoName }}Service(repo {{ .Table.GoName }}Repository) {{ .Table.GoName }}Service {
	return {{ .Table.GoName }}Service{repo: repo}
}

{{- if .Queries.GetAll }}
func (s {{ .Table.GoName }}Service) GetAll{{ .Table.GoPlural }}(ctx context.Context) ([]domain.{{ .Table.GoName }}, error) {
	return s.repo.GetAll{{ .Table.GoPlural }}(ctx)
}
{{- end }}

{{- if .Queries.GetByID }}
func (s {{ .Table.GoName }}Service) Get{{ .Table.GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .Table.GoName }}, error) {
	return s.repo.Get{{ .Table.GoName }}ByID(ctx, id)
}
{{- end }}

{{- if .Queries.Create }}
func (s {{ .Table.GoName }}Service) Create{{ .Table.GoName }}(ctx context.Context, item domain.{{ .Table.GoName }}DB) (domain.{{ .Table.GoName }}, error) {
	return s.repo.Create{{ .Table.GoName }}(ctx, item)
}
{{- end }}

{{- if .Queries.Delete }}
func (s {{ .Table.GoName }}Service) Delete{{ .Table.GoName }}(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete{{ .Table.GoName }}(ctx, id)
}
{{- end }}

{{- if .Queries.DeleteAll }}
func (s {{ .Table.GoName }}Service) DeleteAll{{ .Table.GoPlural }}(ctx context.Context) error {
	return s.repo.DeleteAll{{ .Table.GoPlural }}(ctx)
}
{{- end }}

{{- range .Table.Endpoints }}
func (s {{ $.Table.GoName }}Service) {{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) ({{ if eq .Result "many" }}[]{{ end }}domain.{{ $.Table.GoName }}, error) {
	return s.repo.{{ .Name }}(ctx, params)
}
{{- end }}
`

const repoUtilsTemplate = `package pgrepo
`

const repoTemplate = `package pgrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"{{ .DBImport }}"
	"{{ .Module }}/internal/app/domain"
)

type {{ .Table.GoName }}Repo struct {
	query *db.Queries
}

func New{{ .Table.GoName }}Repo(query *db.Queries) *{{ .Table.GoName }}Repo {
	return &{{ .Table.GoName }}Repo{query: query}
}

{{- if .Queries.GetAll }}
func (r {{ .Table.GoName }}Repo) GetAll{{ .Table.GoPlural }}(ctx context.Context) ([]domain.{{ .Table.GoName }}, error) {
	items, err := r.query.Get{{ .Table.GoPlural }}(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get {{ lower .Table.GoPlural }}: %w", err)
	}
	result := make([]domain.{{ .Table.GoName }}, 0, len(items))
	for _, item := range items {
		result = append(result, domain.New{{ .Table.GoName }}FromDB(item))
	}
	return result, nil
}
{{- end }}

{{- if .Queries.GetByID }}
func (r {{ .Table.GoName }}Repo) Get{{ .Table.GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .Table.GoName }}, error) {
	item, err := r.query.Get{{ .Table.GoName }}ByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.{{ .Table.GoName }}{}, domain.ErrNotFound
		}
		return domain.{{ .Table.GoName }}{}, fmt.Errorf("failed to get {{ lower .Table.GoName }}: %w", err)
	}
	return domain.New{{ .Table.GoName }}FromDB(item), nil
}
{{- end }}

{{- if .Queries.Create }}
func (r {{ .Table.GoName }}Repo) Create{{ .Table.GoName }}(ctx context.Context, item domain.{{ .Table.GoName }}DB) (domain.{{ .Table.GoName }}, error) {
	if err := item.Validate(); err != nil {
		return domain.{{ .Table.GoName }}{}, err
	}
	created, err := r.query.Create{{ .Table.GoName }}(ctx, db.Create{{ .Table.GoName }}Params{
{{- range .Table.CreateCols }}
		{{ .GoName }}: {{ .DBValue }},
{{- end }}
	})
	if err != nil {
		return domain.{{ .Table.GoName }}{}, fmt.Errorf("failed to create {{ lower .Table.GoName }}: %w", err)
	}
	return domain.New{{ .Table.GoName }}FromDB(created), nil
}
{{- end }}

{{- if .Queries.Delete }}
func (r {{ .Table.GoName }}Repo) Delete{{ .Table.GoName }}(ctx context.Context, id uuid.UUID) error {
	if err := r.query.SoftDelete{{ .Table.GoName }}(ctx, id); err != nil {
		return fmt.Errorf("failed to delete {{ lower .Table.GoName }}: %w", err)
	}
	return nil
}
{{- end }}

{{- if .Queries.DeleteAll }}
func (r {{ .Table.GoName }}Repo) DeleteAll{{ .Table.GoPlural }}(ctx context.Context) error {
	if err := r.query.SoftDeleteAll{{ .Table.GoPlural }}(ctx); err != nil {
		return fmt.Errorf("failed to delete {{ lower .Table.GoPlural }}: %w", err)
	}
	return nil
}
{{- end }}

{{- range .Table.Endpoints }}
func (r {{ $.Table.GoName }}Repo) {{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) ({{ if eq .Result "many" }}[]{{ end }}domain.{{ $.Table.GoName }}, error) {
	if err := params.Validate(); err != nil {
		{{- if eq .Result "many" }}
		return nil, err
		{{- else }}
		return domain.{{ $.Table.GoName }}{}, err
		{{- end }}
	}
	items, err := r.query.{{ .Query }}(ctx{{ repoQueryArg . }})
	if err != nil {
		{{- if eq .Result "one" }}
		if errors.Is(err, sql.ErrNoRows) {
			return domain.{{ $.Table.GoName }}{}, domain.ErrNotFound
		}
		{{- end }}
		{{- if eq .Result "many" }}
		return nil, fmt.Errorf("failed to execute {{ .Query }}: %w", err)
		{{- else }}
		return domain.{{ $.Table.GoName }}{}, fmt.Errorf("failed to execute {{ .Query }}: %w", err)
		{{- end }}
	}
	{{- if eq .Result "many" }}
	result := make([]domain.{{ $.Table.GoName }}, 0, len(items))
	for _, item := range items {
		result = append(result, domain.New{{ $.Table.GoName }}FromDB(item))
	}
	return result, nil
	{{- else }}
	return domain.New{{ $.Table.GoName }}FromDB(items), nil
	{{- end }}
}
{{- end }}
`

const httpModelsTemplate = `package httpmodels

import (
{{- if hasImport .Table.CreateCols "time" }}
	"time"
{{- end }}
{{- if hasImport .Table.Columns "uuid" }}
	"github.com/google/uuid"
{{- end }}

	"{{ .Module }}/internal/app/domain"
)

type {{ .Table.GoName }}Request struct {
{{- range .Table.CreateCols }}
	{{ .GoName }} {{ .GoType }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}

func (r {{ .Table.GoName }}Request) ToDomainDB() (domain.{{ .Table.GoName }}DB, error) {
	return domain.New{{ .Table.GoName }}DB(domain.{{ .Table.GoName }}DBData{
{{- range .Table.CreateCols }}
		{{ .GoName }}: r.{{ .GoName }},
{{- end }}
	})
}

type {{ .Table.GoName }}Response struct {
{{- range .Table.Columns }}
	{{ .GoName }} {{ .GoType }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}

func New{{ .Table.GoName }}Response(item domain.{{ .Table.GoName }}) {{ .Table.GoName }}Response {
	return {{ .Table.GoName }}Response{
{{- range .Table.Columns }}
		{{ .GoName }}: item.{{ .GoName }},
{{- end }}
	}
}

{{- range .Table.Endpoints }}
{{- if endpointNeedsBody . }}
type {{ .Name }}Request struct {
{{- range .BodyParams }}
	{{ .GoName }} {{ .GoType }} ` + "`json:\"{{ .JSONName }}\"`" + `
{{- end }}
}
{{- end }}
{{- end }}
`

const httpServerTemplate = `package httpserver

type HttpServer struct {
{{- range .Tables }}
	{{ .Singular }}Service {{ .GoName }}Service
{{- end }}
}

func NewHttpServer(
{{- range .Tables }}
	{{ .Singular }}Service {{ .GoName }}Service,
{{- end }}
) HttpServer {
	return HttpServer{
{{- range .Tables }}
		{{ .Singular }}Service: {{ .Singular }}Service,
{{- end }}
	}
}
`

const httpServerInterfacesTemplate = `package httpserver

import (
	"context"

	"github.com/google/uuid"
	"{{ .Module }}/internal/app/domain"
)

{{ range .Tables }}
{{- $table := . }}
type {{ .GoName }}Service interface {
	GetAll{{ .GoPlural }}(ctx context.Context) ([]domain.{{ .GoName }}, error)
	Get{{ .GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .GoName }}, error)
	Create{{ .GoName }}(ctx context.Context, item domain.{{ .GoName }}DB) (domain.{{ .GoName }}, error)
	Delete{{ .GoName }}(ctx context.Context, id uuid.UUID) error
	DeleteAll{{ .GoPlural }}(ctx context.Context) error
{{- range .Endpoints }}
	{{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) ({{ if eq .Result "many" }}[]{{ end }}domain.{{ $table.GoName }}, error)
{{- end }}
}
{{ end }}
`

const httpHandlersTemplate = `package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
{{- if tableEndpointNeedsStrconv .Table }}
	"strconv"
{{- end }}
{{- if tableEndpointNeedsTime .Table }}
	"time"
{{- end }}

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"{{ .Module }}/internal/app/common/server"
	"{{ .Module }}/internal/app/domain"
	"{{ .Module }}/internal/app/transport/httpmodels"
)

{{- if .Queries.GetAll }}
func (h HttpServer) GetAll{{ .Table.GoPlural }}(w http.ResponseWriter, r *http.Request) {
	items, err := h.{{ .Table.Singular }}Service.GetAll{{ .Table.GoPlural }}(r.Context())
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	response := make([]httpmodels.{{ .Table.GoName }}Response, 0, len(items))
	for _, item := range items {
		response = append(response, httpmodels.New{{ .Table.GoName }}Response(item))
	}
	server.RespondOK(response, w, r)
}
{{- end }}

{{- if .Queries.GetByID }}
func (h HttpServer) Get{{ .Table.GoName }}ByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	item, err := h.{{ .Table.Singular }}Service.Get{{ .Table.GoName }}ByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(httpmodels.New{{ .Table.GoName }}Response(item), w, r)
}
{{- end }}

{{- if .Queries.Create }}
func (h HttpServer) Create{{ .Table.GoName }}(w http.ResponseWriter, r *http.Request) {
	var request httpmodels.{{ .Table.GoName }}Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	input, err := request.ToDomainDB()
	if err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	item, err := h.{{ .Table.Singular }}Service.Create{{ .Table.GoName }}(r.Context(), input)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(httpmodels.New{{ .Table.GoName }}Response(item), w, r)
}
{{- end }}

{{- if .Queries.Delete }}
func (h HttpServer) Delete{{ .Table.GoName }}(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	if err := h.{{ .Table.Singular }}Service.Delete{{ .Table.GoName }}(r.Context(), id); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}
{{- end }}

{{- if .Queries.DeleteAll }}
func (h HttpServer) DeleteAll{{ .Table.GoPlural }}(w http.ResponseWriter, r *http.Request) {
	if err := h.{{ .Table.Singular }}Service.DeleteAll{{ .Table.GoPlural }}(r.Context()); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}
{{- end }}

{{- range .Table.Endpoints }}
func (h HttpServer) {{ .Name }}(w http.ResponseWriter, r *http.Request) {
	var params domain.{{ .Name }}Params
	{{- if endpointNeedsBody . }}
	var body httpmodels.{{ .Name }}Request
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	{{- end }}
	{{- range .Params }}
	{{ handlerParamRead . }}
	{{- end }}
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.{{ $.Table.Singular }}Service.{{ .Name }}(r.Context(), params)
	if err != nil {
		{{- if eq .Result "one" }}
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("not-found", err, w, r)
			return
		}
		{{- end }}
		server.RespondWithError(err, w, r)
		return
	}
	{{- if eq .Result "many" }}
	response := make([]httpmodels.{{ $.Table.GoName }}Response, 0, len(items))
	for _, item := range items {
		response = append(response, httpmodels.New{{ $.Table.GoName }}Response(item))
	}
	server.RespondOK(response, w, r)
	{{- else }}
	server.RespondOK(httpmodels.New{{ $.Table.GoName }}Response(items), w, r)
	{{- end }}
}
{{- end }}
`

const httpUtilsTemplate = `package httpserver
`

const httpServerEndpointsTestTemplate = `package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
{{- if tablesNeedTime .Tables }}
	"time"
{{- end }}

	"github.com/gorilla/mux"
	"github.com/google/uuid"
	"{{ .Module }}/internal/app/domain"
)

{{- range .Tables }}
{{- $table := . }}
type fake{{ .GoName }}Service struct{}

func sample{{ .GoName }}() domain.{{ .GoName }} {
	return {{ sampleDomain . }}
}

func (fake{{ .GoName }}Service) GetAll{{ .GoPlural }}(ctx context.Context) ([]domain.{{ .GoName }}, error) {
	return []domain.{{ .GoName }}{sample{{ .GoName }}()}, nil
}

func (fake{{ .GoName }}Service) Get{{ .GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .GoName }}, error) {
	return sample{{ .GoName }}(), nil
}

func (fake{{ .GoName }}Service) Create{{ .GoName }}(ctx context.Context, item domain.{{ .GoName }}DB) (domain.{{ .GoName }}, error) {
	return sample{{ .GoName }}(), nil
}

func (fake{{ .GoName }}Service) Delete{{ .GoName }}(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (fake{{ .GoName }}Service) DeleteAll{{ .GoPlural }}(ctx context.Context) error {
	return nil
}

{{- range .Endpoints }}
func (fake{{ $table.GoName }}Service) {{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) ({{ if eq .Result "many" }}[]{{ end }}domain.{{ $table.GoName }}, error) {
	{{- if eq .Result "many" }}
	return []domain.{{ $table.GoName }}{sample{{ $table.GoName }}()}, nil
	{{- else }}
	return sample{{ $table.GoName }}(), nil
	{{- end }}
}

{{- end }}
{{- end }}

func testRouter() *mux.Router {
	httpServer := NewHttpServer(
{{- range .Tables }}
		fake{{ .GoName }}Service{},
{{- end }}
	)
	router := mux.NewRouter()
{{- range .Tables }}
	router.HandleFunc("{{ .RouteBase }}", httpServer.GetAll{{ .GoPlural }}).Methods(http.MethodGet)
	router.HandleFunc("{{ .RouteBase }}", httpServer.Create{{ .GoName }}).Methods(http.MethodPost)
	router.HandleFunc("{{ .RouteBase }}", httpServer.DeleteAll{{ .GoPlural }}).Methods(http.MethodDelete)
{{- range .Endpoints }}
	router.HandleFunc("{{ .Path }}", httpServer.{{ .Name }}).Methods("{{ .Method }}")
{{- end }}
	router.HandleFunc("{{ .RouteBase }}/{id}", httpServer.Get{{ .GoName }}ByID).Methods(http.MethodGet)
	router.HandleFunc("{{ .RouteBase }}/{id}", httpServer.Delete{{ .GoName }}).Methods(http.MethodDelete)
{{- end }}
	return router
}

func TestGeneratedEndpoints(t *testing.T) {
	router := testRouter()
	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
{{- range .Tables }}
		{name: "get all {{ .Name }}", method: http.MethodGet, url: "{{ .RouteBase }}"},
		{name: "create {{ .Singular }}", method: http.MethodPost, url: "{{ .RouteBase }}", body: ` + "`{{ createJSONBody .CreateCols }}`" + `},
		{name: "delete all {{ .Name }}", method: http.MethodDelete, url: "{{ .RouteBase }}"},
{{- range .Endpoints }}
		{name: "{{ .Name }}", method: "{{ .Method }}", url: "{{ testURL . }}", body: ` + "`{{ endpointJSONBody . }}`" + `},
{{- end }}
		{name: "get {{ .Singular }} by id", method: http.MethodGet, url: "{{ .RouteBase }}/00000000-0000-0000-0000-000000000001"},
		{name: "delete {{ .Singular }}", method: http.MethodDelete, url: "{{ .RouteBase }}/00000000-0000-0000-0000-000000000001"},
{{- end }}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
			}
		})
	}
}
`

const appMainTemplate = `package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"{{ .Module }}/internal/app/config"
	"{{ .DBImport }}"
	"{{ .Module }}/internal/app/repository/pgrepo"
	"{{ .Module }}/internal/app/services"
	"{{ .Module }}/internal/app/transport/httpserver"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func Dial(dsn string) (*sql.DB, *db.Queries, error) {
	if dsn == "" {
		return nil, nil, errors.New("no postgres DSN provided")
	}
	dbase, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("sql.Open failed: %w", err)
	}
	dbase.SetMaxIdleConns(10)
	dbase.SetMaxOpenConns(10)
	dbase.SetConnMaxLifetime(time.Minute)
	return dbase, db.New(dbase), nil
}

func run() error {
	cfg := config.Read()
	dbase, queries, err := Dial(cfg.DB_DSN)
	if err != nil {
		return err
	}
	defer dbase.Close()

{{- range .Tables }}
	{{ .Singular }}Repo := pgrepo.New{{ .GoName }}Repo(queries)
	{{ .Singular }}Service := services.New{{ .GoName }}Service({{ .Singular }}Repo)
{{- end }}
	httpServer := httpserver.NewHttpServer(
{{- range .Tables }}
		{{ .Singular }}Service,
{{- end }}
	)

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Generated API"))
	}).Methods(http.MethodGet)
{{- range .Tables }}
	router.HandleFunc("{{ .RouteBase }}", httpServer.GetAll{{ .GoPlural }}).Methods(http.MethodGet)
	router.HandleFunc("{{ .RouteBase }}", httpServer.Create{{ .GoName }}).Methods(http.MethodPost)
	router.HandleFunc("{{ .RouteBase }}", httpServer.DeleteAll{{ .GoPlural }}).Methods(http.MethodDelete)
{{- range .Endpoints }}
	router.HandleFunc("{{ .Path }}", httpServer.{{ .Name }}).Methods("{{ .Method }}")
{{- end }}
	router.HandleFunc("{{ .RouteBase }}/{id}", httpServer.Get{{ .GoName }}ByID).Methods(http.MethodGet)
	router.HandleFunc("{{ .RouteBase }}/{id}", httpServer.Delete{{ .GoName }}).Methods(http.MethodDelete)
{{- end }}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: router}
	stopped := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		close(stopped)
	}()

	log.Printf("Starting HTTP server on %s", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	<-stopped
	return nil
}
`

const makefileTemplate = `-include .env

APP_NAME ?= app
BUILD_DIR ?= ./bin
DB_SCRIPT ?= init_db.sh
DB_NAME ?= app_db
DB_USER ?= app_user
DB_PASS ?= app_password
DB_DRIVER ?= postgres
MIGRATIONS_DIR ?= ./internal/sql/migrations
DB_DSN ?= postgres://$(DB_USER):$(DB_PASS)@localhost:5432/$(DB_NAME)?sslmode=disable
HTTP_ADDR ?= :8080
DEBUG_ERRORS ?= 1
GOCACHE ?= $(CURDIR)/.cache/go-build

export

.PHONY: build run test clean db migrate-status migrate-up migrate-down migrate-create

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd

run:
	@mkdir -p $(BUILD_DIR) && \
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd && \
	HTTP_ADDR=$(HTTP_ADDR) \
	DB_DSN=$(DB_DSN) \
	DEBUG_ERRORS=$(DEBUG_ERRORS) \
	$(BUILD_DIR)/$(APP_NAME)

test:
	go test -race -v ./...

clean:
	rm -rf $(BUILD_DIR)

db:
	@test -f $(DB_SCRIPT) || { echo "Ошибка: $(DB_SCRIPT) отсутствует"; exit 1; }
	@chmod +x $(DB_SCRIPT)
	@./$(DB_SCRIPT)

migrate-status:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) status

migrate-up:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) up

migrate-down:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) down

migrate-create:
	@read -p "Название миграции: " name; \
	goose -dir $(MIGRATIONS_DIR) create $$name sql
`

const initDBTemplate = `#!/usr/bin/env bash
set -euo pipefail

if [[ -f .env ]]; then
	set -a
	source .env
	set +a
fi

: "${DB_NAME:=app_db}"
: "${DB_USER:=app_user}"
: "${DB_PASS:=app_password}"
: "${DB_ADMIN_DB:=postgres}"
: "${USE_SUDO_POSTGRES:=0}"

if [[ "$USE_SUDO_POSTGRES" == "1" ]]; then
	PSQL_ADMIN=(sudo -u postgres psql -d "$DB_ADMIN_DB")
else
	PSQL_ADMIN=(psql -d "$DB_ADMIN_DB")
fi

sql_literal() {
	printf "'%s'" "${1//\'/\'\'}"
}

echo "Настройка базы данных '$DB_NAME' и пользователя '$DB_USER'..."

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_roles WHERE rolname = $(sql_literal "$DB_USER")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
	echo "Пользователь '$DB_USER' создан."
else
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "ALTER USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
	echo "Пользователь '$DB_USER' уже существует, пароль обновлен."
fi

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_database WHERE datname = $(sql_literal "$DB_NAME")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$DB_NAME\" OWNER \"$DB_USER\";"
	echo "База данных '$DB_NAME' создана."
else
	echo "База данных '$DB_NAME' уже существует."
fi

"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE "$DB_NAME" TO "$DB_USER";
ALTER DATABASE "$DB_NAME" OWNER TO "$DB_USER";
SQL

if [[ "$USE_SUDO_POSTGRES" == "1" ]]; then
	PSQL_TARGET=(sudo -u postgres psql -d "$DB_NAME")
else
	PSQL_TARGET=(psql -d "$DB_NAME")
fi

"${PSQL_TARGET[@]}" -v ON_ERROR_STOP=1 <<SQL
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO "$DB_USER";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "$DB_USER";
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO "$DB_USER";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO "$DB_USER";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO "$DB_USER";
SQL

echo "Готово: база '$DB_NAME' доступна пользователю '$DB_USER'."
`
