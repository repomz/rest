package generator

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

{{- if tableHasServiceMethods .Table }}
import (
	"context"
{{- if tableServiceNeedsTime .Table }}
	"time"
{{- end }}
{{- if tableServiceNeedsUUID .Table }}

	"github.com/google/uuid"
{{- end }}
	"{{ .Module }}/internal/app/domain"
)
{{- end }}

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
	{{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) {{ endpointReturn . }}
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
{{ end }}

{{ if .Queries.GetByID }}
func (s {{ .Table.GoName }}Service) Get{{ .Table.GoName }}ByID(ctx context.Context, id uuid.UUID) (domain.{{ .Table.GoName }}, error) {
	return s.repo.Get{{ .Table.GoName }}ByID(ctx, id)
}
{{ end }}

{{ if .Queries.Create }}
func (s {{ .Table.GoName }}Service) Create{{ .Table.GoName }}(ctx context.Context, item domain.{{ .Table.GoName }}DB) (domain.{{ .Table.GoName }}, error) {
	return s.repo.Create{{ .Table.GoName }}(ctx, item)
}
{{ end }}

{{ if .Queries.Delete }}
func (s {{ .Table.GoName }}Service) Delete{{ .Table.GoName }}(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete{{ .Table.GoName }}(ctx, id)
}
{{ end }}

{{ if .Queries.DeleteAll }}
func (s {{ .Table.GoName }}Service) DeleteAll{{ .Table.GoPlural }}(ctx context.Context) error {
	return s.repo.DeleteAll{{ .Table.GoPlural }}(ctx)
}
{{ end }}

{{ range .Table.Endpoints }}
func (s {{ $.Table.GoName }}Service) {{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) {{ endpointReturn . }} {
	return s.repo.{{ .Name }}(ctx, params)
}
{{ end }}
`

const repoTemplate = `package pgrepo

import (
{{- if tableHasServiceMethods .Table }}
	"context"
{{- end }}
{{- if tableRepoNeedsSQL .Table }}
	"database/sql"
{{- end }}
{{- if tableRepoNeedsErrors .Table }}
	"errors"
{{- end }}
{{- if tableHasServiceMethods .Table }}
	"fmt"
{{- end }}
{{- if tableRepoNeedsTime .Table }}
	"time"
{{- end }}
{{- if tableRepoNeedsUUID .Table }}

	"github.com/google/uuid"
{{- end }}
	"{{ .DBImport }}"
{{- if tableHasServiceMethods .Table }}
	"{{ .Module }}/internal/app/domain"
{{- end }}
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
{{ end }}

{{ if .Queries.GetByID }}
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
{{ end }}

{{ if .Queries.Create }}
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
{{ end }}

{{ if .Queries.Delete }}
func (r {{ .Table.GoName }}Repo) Delete{{ .Table.GoName }}(ctx context.Context, id uuid.UUID) error {
	if err := r.query.SoftDelete{{ .Table.GoName }}(ctx, id); err != nil {
		return fmt.Errorf("failed to delete {{ lower .Table.GoName }}: %w", err)
	}
	return nil
}
{{ end }}

{{ if .Queries.DeleteAll }}
func (r {{ .Table.GoName }}Repo) DeleteAll{{ .Table.GoPlural }}(ctx context.Context) error {
	if err := r.query.SoftDeleteAll{{ .Table.GoPlural }}(ctx); err != nil {
		return fmt.Errorf("failed to delete {{ lower .Table.GoPlural }}: %w", err)
	}
	return nil
}
{{ end }}

{{ range .Table.Endpoints }}
func (r {{ $.Table.GoName }}Repo) {{ .Name }}(ctx context.Context, params domain.{{ .Name }}Params) {{ endpointReturn . }} {
	if err := params.Validate(); err != nil {
		{{- if .IsExec }}
		return err
		{{- else }}
		return {{ .ZeroValue }}, err
		{{- end }}
	}
	{{- if .IsExec }}
	if err := r.query.{{ .Query }}(ctx{{ repoQueryArg . }}); err != nil {
		return fmt.Errorf("failed to execute {{ .Query }}: %w", err)
	}
	return nil
	{{- else }}
	items, err := r.query.{{ .Query }}(ctx{{ repoQueryArg . }})
	if err != nil {
		{{- if and (eq .Result "one") .DomainResponse }}
		if errors.Is(err, sql.ErrNoRows) {
			return {{ .ZeroValue }}, domain.ErrNotFound
		}
		{{- end }}
		return {{ .ZeroValue }}, fmt.Errorf("failed to execute {{ .Query }}: %w", err)
	}
	{{- if .DomainResponse }}
		{{- if eq .Result "many" }}
	result := make([]domain.{{ $.Table.GoName }}, 0, len(items))
	for _, item := range items {
		result = append(result, domain.New{{ $.Table.GoName }}FromDB(item))
	}
	return result, nil
		{{- else }}
	return domain.New{{ $.Table.GoName }}FromDB(items), nil
		{{- end }}
	{{- else }}
	return items, nil
	{{- end }}
	{{- end }}
}
{{ end }}
`
