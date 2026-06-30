package generator

const httpModelsTemplate = `package httpmodels

import (
{{- if hasImport .Table.Columns "time" }}
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
{{ end }}
`

const httpServerTemplate = `package httpserver

type HttpServer struct {
{{- range .Tables }}
	{{ .Singular }}Service {{ .GoName }}Service
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
	tokenService TokenService
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
	basicAuth BasicAuthConfig
{{- end }}
}

func NewHttpServer(
{{- range .Tables }}
	{{ .Singular }}Service {{ .GoName }}Service,
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
	tokenService TokenService,
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
basicAuth BasicAuthConfig,
{{- end }}
) HttpServer {
	return HttpServer{
{{- range .Tables }}
		{{ .Singular }}Service: {{ .Singular }}Service,
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
		tokenService: tokenService,
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
		basicAuth: basicAuth,
{{- end }}
	}
}
`

const httpServerInterfacesTemplate = `package httpserver

{{- if anyServiceNeedsImports .Tables }}
import (
	"context"
{{- if anyServiceNeedsTime .Tables }}
	"time"
{{- end }}
{{- if anyServiceNeedsUUID .Tables }}

	"github.com/google/uuid"
{{- end }}
	"{{ .Module }}/internal/app/domain"
)
{{- end }}

{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
type TokenService interface {
	GenerateToken(user domain.{{ .Features.Auth.UserModel }}) (string, string, error)
	GetUser(token string) (domain.{{ .Features.Auth.UserModel }}, error)
}
{{- end }}

{{ range .Tables }}
{{- $table := . }}
type {{ .GoName }}Service interface {
{{- if .Queries.GetAll }}
	GetAll{{ .GoPlural }}(ctx context.Context) ([]domain.{{ .GoName }}, error)
{{- end }}
{{- if .Queries.GetByID }}
	Get{{ .GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .GoName }}, error)
{{- end }}
{{- if .Queries.Create }}
	Create{{ .GoName }}(ctx context.Context, item domain.{{ .GoName }}DB) (domain.{{ .GoName }}, error)
{{- end }}
{{- if .Queries.Delete }}
	Delete{{ .GoName }}(ctx context.Context, id uuid.UUID) error
{{- end }}
{{- if .Queries.DeleteAll }}
	DeleteAll{{ .GoPlural }}(ctx context.Context) error
{{- end }}
{{- range .Endpoints }}
	{{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) {{ endpointReturn . }}
{{- end }}
}
{{ end }}
`

const httpHandlersTemplate = `package httpserver
{{- if tableHasHandlers .Table }}

import (
{{ if tableHandlersNeedJSON .Table -}}
	"encoding/json"
{{ end -}}
{{ if tableHandlersNeedErrors .Table -}}
	"errors"
{{ end -}}
	"net/http"
{{ if tableEndpointNeedsStrconv .Table -}}
	"strconv"
{{ end -}}
{{ if tableEndpointNeedsTime .Table -}}
	"time"
{{ end -}}
{{ if or (tableHandlersNeedMux .Table) (tableHandlersNeedUUID .Table) }}
	"github.com/gorilla/mux"
{{ if tableHandlersNeedUUID .Table -}}
	"github.com/google/uuid"
{{ end -}}
{{ end -}}
	"{{ .Module }}/internal/app/common/server"
	"{{ .Module }}/internal/app/domain"
	"{{ .Module }}/internal/app/transport/httpmodels"
)
{{- end }}

{{ if .Queries.GetAll }}
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
{{ end }}

{{ if .Queries.GetByID }}
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
{{ end }}

{{ if .Queries.Create }}
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
{{ end }}

{{ if .Queries.Delete }}
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
{{ end }}

{{ if .Queries.DeleteAll }}
func (h HttpServer) DeleteAll{{ .Table.GoPlural }}(w http.ResponseWriter, r *http.Request) {
	if err := h.{{ .Table.Singular }}Service.DeleteAll{{ .Table.GoPlural }}(r.Context()); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}
{{ end }}

{{ range .Table.Endpoints }}
func (h HttpServer) {{ .Name }}(w http.ResponseWriter, r *http.Request) {
	var params domain.{{ .Name }}Params
	{{- if endpointNeedsBody . }}
	var body httpmodels.{{ .Name }}Request
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
{{ handlerBodyParamReads . }}
	{{- end }}
{{ handlerNonBodyParamReads . }}
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	{{- if .IsExec }}
	if err := h.{{ $.Table.Singular }}Service.{{ .Name }}(r.Context(), params); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"ok": true}, w, r)
	{{- else }}
	items, err := h.{{ $.Table.Singular }}Service.{{ .Name }}(r.Context(), params)
	if err != nil {
		{{- if and (eq .Result "one") .DomainResponse }}
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("not-found", err, w, r)
			return
		}
		{{- end }}
		server.RespondWithError(err, w, r)
		return
	}
	{{- if .DomainResponse }}
		{{- if eq .Result "many" }}
	response := make([]httpmodels.{{ $.Table.GoName }}Response, 0, len(items))
	for _, item := range items {
		response = append(response, httpmodels.New{{ $.Table.GoName }}Response(item))
	}
	server.RespondOK(response, w, r)
		{{- else }}
	server.RespondOK(httpmodels.New{{ $.Table.GoName }}Response(items), w, r)
		{{- end }}
	{{- else }}
	server.RespondOK(items, w, r)
	{{- end }}
	{{- end }}
}
{{ end }}
`

const httpHandlersTestTemplate = `package httpserver
{{- if tableHasHandlers .Table }}

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
{{ if tableTestNeedsTime .Table -}}
	"time"
{{ end }}
	"github.com/gorilla/mux"
{{ if tableServiceNeedsUUID .Table -}}
	"github.com/google/uuid"
{{ end -}}
	"{{ .Module }}/internal/app/domain"
)

type fake{{ .Table.GoName }}Service struct{}

func sample{{ .Table.GoName }}() domain.{{ .Table.GoName }} {
	return {{ sampleDomain .Table }}
}

{{ if .Queries.GetAll }}
func (fake{{ .Table.GoName }}Service) GetAll{{ .Table.GoPlural }}(ctx context.Context) ([]domain.{{ .Table.GoName }}, error) {
	return []domain.{{ .Table.GoName }}{sample{{ .Table.GoName }}()}, nil
}
{{ end }}

{{ if .Queries.GetByID }}
func (fake{{ .Table.GoName }}Service) Get{{ .Table.GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .Table.GoName }}, error) {
	return sample{{ .Table.GoName }}(), nil
}
{{ end }}

{{ if .Queries.Create }}
func (fake{{ .Table.GoName }}Service) Create{{ .Table.GoName }}(ctx context.Context, item domain.{{ .Table.GoName }}DB) (domain.{{ .Table.GoName }}, error) {
	return sample{{ .Table.GoName }}(), nil
}
{{ end }}

{{ if .Queries.Delete }}
func (fake{{ .Table.GoName }}Service) Delete{{ .Table.GoName }}(ctx context.Context, id uuid.UUID) error {
	return nil
}
{{ end }}

{{ if .Queries.DeleteAll }}
func (fake{{ .Table.GoName }}Service) DeleteAll{{ .Table.GoPlural }}(ctx context.Context) error {
	return nil
}
{{ end }}

{{ range .Table.Endpoints }}
func (fake{{ $.Table.GoName }}Service) {{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) {{ endpointReturn . }} {
	return {{ .SampleReturn }}
}
{{ end }}

{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
type fake{{ .Table.GoName }}TokenService struct{}

func (fake{{ .Table.GoName }}TokenService) GenerateToken(user domain.{{ .Features.Auth.UserModel }}) (string, string, error) {
	return "token", "", nil
}

func (fake{{ .Table.GoName }}TokenService) GetUser(token string) (domain.{{ .Features.Auth.UserModel }}, error) {
	return domain.{{ .Features.Auth.UserModel }}{}, nil
}
{{- end }}

func test{{ .Table.GoName }}HandlersRouter() *mux.Router {
	httpServer := NewHttpServer(
{{- range .Tables }}
{{- if eq .Name $.Table.Name }}
		fake{{ .GoName }}Service{},
{{- else }}
		nil,
{{- end }}
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
		fake{{ .Table.GoName }}TokenService{},
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
		BasicAuthConfig{},
{{- end }}
	)
	router := mux.NewRouter()
{{- if .Queries.GetAll }}
	router.HandleFunc("{{ .Table.RouteBase }}", httpServer.GetAll{{ .Table.GoPlural }}).Methods(http.MethodGet)
{{- end }}
{{- if .Queries.Create }}
	router.HandleFunc("{{ .Table.RouteBase }}", httpServer.Create{{ .Table.GoName }}).Methods(http.MethodPost)
{{- end }}
{{- if .Queries.DeleteAll }}
	router.HandleFunc("{{ .Table.RouteBase }}", httpServer.DeleteAll{{ .Table.GoPlural }}).Methods(http.MethodDelete)
{{- end }}
{{- range .Table.Endpoints }}
	router.HandleFunc("{{ .Path }}", httpServer.{{ .Name }}).Methods("{{ .Method }}")
{{- end }}
{{- if .Queries.GetByID }}
	router.HandleFunc("{{ .Table.RouteBase }}/{id}", httpServer.Get{{ .Table.GoName }}ByID).Methods(http.MethodGet)
{{- end }}
{{- if .Queries.Delete }}
	router.HandleFunc("{{ .Table.RouteBase }}/{id}", httpServer.Delete{{ .Table.GoName }}).Methods(http.MethodDelete)
{{- end }}
	return router
}

func Test{{ .Table.GoName }}Handlers(t *testing.T) {
	router := test{{ .Table.GoName }}HandlersRouter()
	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
{{- if .Queries.GetAll }}
		{name: "get all {{ .Table.Name }}", method: http.MethodGet, url: "{{ .Table.RouteBase }}"},
{{- end }}
{{- if .Queries.Create }}
		{name: "create {{ .Table.Singular }}", method: http.MethodPost, url: "{{ .Table.RouteBase }}", body: ` + "`{{ createJSONBody .Table.CreateCols }}`" + `},
{{- end }}
{{- if .Queries.DeleteAll }}
		{name: "delete all {{ .Table.Name }}", method: http.MethodDelete, url: "{{ .Table.RouteBase }}"},
{{- end }}
{{- range .Table.Endpoints }}
		{name: "{{ .Name }}", method: "{{ .Method }}", url: "{{ testURL . }}", body: ` + "`{{ endpointJSONBody . }}`" + `},
{{- end }}
{{- if .Queries.GetByID }}
		{name: "get {{ .Table.Singular }} by id", method: http.MethodGet, url: "{{ .Table.RouteBase }}/00000000-0000-0000-0000-000000000001"},
{{- end }}
{{- if .Queries.Delete }}
		{name: "delete {{ .Table.Singular }}", method: http.MethodDelete, url: "{{ .Table.RouteBase }}/00000000-0000-0000-0000-000000000001"},
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
{{- end }}
`
