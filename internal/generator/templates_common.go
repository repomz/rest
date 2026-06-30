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

func Unauthorised(slug string, err error, w http.ResponseWriter, r *http.Request) {
	httpRespondWithError(err, slug, w, r, "Unauthorised", http.StatusUnauthorized)
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
		HTTPAddr: {{ printf "%q" (httpAddr .Features.HTTP.Host .Features.HTTP.Port) }},
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
