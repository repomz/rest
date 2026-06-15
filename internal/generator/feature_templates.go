package generator

const httpMiddlewareTemplate = `package middleware

import (
	{{- if and .Features.HTTP.Recovery .Features.HTTP.RecoveryExposeDetails }}
	"fmt"
	{{- end }}
	{{- if .Features.HTTP.Recovery }}
	"log"
	{{- end }}
	"net/http"
	{{- if .Features.HTTP.RequestID }}
	"github.com/google/uuid"
	{{- end }}
)

{{- if .Features.HTTP.CORS }}
func CORS(next http.Handler) http.Handler {
	allowed := map[string]bool{
{{- range .Features.HTTP.AllowOrigins }}
		{{ printf "%q" . }}: true,
{{- end }}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed["*"] {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", {{ printf "%q" (join .Features.HTTP.AllowHeaders ", ") }})
		w.Header().Set("Access-Control-Allow-Methods", {{ printf "%q" (join .Features.HTTP.AllowMethods ", ") }})
		{{- if .Features.HTTP.ExposeHeaders }}
		w.Header().Set("Access-Control-Expose-Headers", {{ printf "%q" (join .Features.HTTP.ExposeHeaders ", ") }})
		{{- end }}
		{{- if .Features.HTTP.AllowCredentials }}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		{{- end }}
		{{- if .Features.HTTP.CORSMaxAge }}
		w.Header().Set("Access-Control-Max-Age", {{ printf "%q" (printf "%d" (durationSeconds .Features.HTTP.CORSMaxAge)) }})
		{{- end }}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
{{- end }}

{{- if .Features.HTTP.Recovery }}
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v", recovered)
				message := http.StatusText(http.StatusInternalServerError)
				{{- if .Features.HTTP.RecoveryExposeDetails }}
				message = fmt.Sprintf("%s: %v", message, recovered)
				{{- end }}
				http.Error(w, message, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
{{- end }}

{{- if .Features.HTTP.RequestID }}
func RequestID(next http.Handler) http.Handler {
	const header = {{ printf "%q" (defaultString .Features.HTTP.RequestIDHeader "X-Request-ID") }}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(header)
		if id == "" {
			id = uuid.NewString()
		}
		r.Header.Set(header, id)
		w.Header().Set(header, id)
		next.ServeHTTP(w, r)
	})
}
{{- end }}

{{- if gt .Features.HTTP.MaxBodyBytes 0 }}
func MaxBodyBytes(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
{{- end }}
`

const loggingTemplate = `package logging

import (
	"os"
	"path/filepath"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
{{- if and (eq .Features.Logging.OutputType "file") .Features.Logging.Rotation }}
	"gopkg.in/natefinch/lumberjack.v2"
{{- end }}
)

func New() (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.Set({{ printf "%q" .Features.Logging.Level }}); err != nil {
		return nil, err
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	var encoder zapcore.Encoder
	if {{ printf "%q" .Features.Logging.Format }} == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}
	var output zapcore.WriteSyncer
	switch {{ printf "%q" .Features.Logging.OutputType }} {
	case "stderr":
		output = zapcore.AddSync(os.Stderr)
	case "file":
		path := {{ printf "%q" .Features.Logging.OutputFile }}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		{{- if .Features.Logging.Rotation }}
		output = zapcore.AddSync(&lumberjack.Logger{
			Filename: path,
			MaxSize: {{ .Features.Logging.MaxSizeMB }},
			MaxBackups: {{ .Features.Logging.MaxBackups }},
			MaxAge: {{ .Features.Logging.MaxAgeDays }},
			Compress: {{ .Features.Logging.Compress }},
		})
		{{- else }}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil { return nil, err }
		output = zapcore.AddSync(file)
		{{- end }}
	default:
		output = zapcore.AddSync(os.Stdout)
	}
	logger := zap.New(zapcore.NewCore(encoder, output, level))
	return logger.With(
{{- range $key, $value := .Features.Logging.Fields }}
		zap.String({{ printf "%q" $key }}, {{ printf "%q" $value }}),
{{- end }}
	), nil
}

func Middleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("http request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			{{- if .Features.HTTP.RequestID }}
			zap.String("request_id", r.Header.Get({{ printf "%q" (defaultString .Features.HTTP.RequestIDHeader "X-Request-ID") }})),
			{{- end }}
			zap.Int("status", recorder.status),
			zap.Int("response_bytes", recorder.bytes),
			zap.Duration("duration", time.Since(started)),
		)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes int
}

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}
`

const swaggerTemplate = `package httpserver

import "net/http"

const swaggerSpec = {{ printf "%q" .OpenAPI }}

func SwaggerSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write([]byte(swaggerSpec))
}

{{- if .Features.OpenAPI.WithUI }}
func SwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(` + "`" + `<!doctype html>
<html><head><title>Generated REST API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:{{ printf "%q" (routePath .Features.HTTP.BasePath (defaultString .Features.OpenAPI.SpecPath "/swagger/openapi.yaml")) }},dom_id:'#swagger-ui'});</script>
</body></html>` + "`" + `))
}
{{- end }}
`
