package appgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/repomz/rest/internal/generator"
)

type MongoFeature struct{}

func (MongoFeature) Name() string { return "mongo" }

func (MongoFeature) Enabled(ctx Context) bool {
	return ctx.Config.Rest.Mongo.Bool() && ctx.Config.Mongo != nil && !(SQLFeature{}).Enabled(ctx)
}

func (MongoFeature) Generate(ctx Context) error {
	models, err := discoverMongoOpenAPIModels(ctx)
	if err != nil {
		return err
	}
	features := mongoFeatureOptions(ctx, models)
	module := ctx.Config.Rest.Module
	if module == "" {
		module = "generated-mongo-api"
	}
	swagger := generator.BuildMongoOpenAPISpec(module, features)
	files := map[string]string{
		"cmd/main.go": mongoMainSource(ctx, models, swagger),
	}
	openAPIOutput := ctx.Config.Rest.OpenAPI.Output
	if openAPIOutput == "" {
		openAPIOutput = "docs/swagger.yaml"
	}
	files[openAPIOutput] = swagger
	if ctx.Config.Rest.Features.Env.Enabled.Bool() {
		envPath := ctx.Config.Rest.Features.Env.Output
		if envPath == "" {
			envPath = ".env.example"
		}
		files[envPath] = fmt.Sprintf("MONGO_URI=mongodb://localhost:27017\nHTTP_ADDR=%s\n", mongoHTTPAddr(ctx))
	}
	for path, content := range files {
		target := filepath.Join(ctx.ProjectDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func mongoFeatureOptions(ctx Context, models []generator.MongoModel) generator.FeatureOptions {
	return generator.FeatureOptions{
		HTTP: generator.HTTPFeatures{
			BasePath:   ctx.Config.Rest.HTTP.BasePath,
			Host:       ctx.Config.Rest.HTTP.Host,
			Port:       ctx.Config.Rest.HTTP.Port,
			Health:     ctx.Config.Rest.HTTP.Health.Enabled.Bool(),
			HealthPath: ctx.Config.Rest.HTTP.Health.Path,
		},
		Auth: authFeatures(ctx.Config),
		OpenAPI: generator.OpenAPIFeatures{
			Enabled:     ctx.Config.Rest.OpenAPI.Enabled.Bool(),
			Output:      ctx.Config.Rest.OpenAPI.Output,
			WithUI:      ctx.Config.Rest.OpenAPI.WithUI.Bool(),
			Title:       ctx.Config.Rest.OpenAPI.Title,
			Version:     ctx.Config.Rest.OpenAPI.Version,
			Description: ctx.Config.Rest.OpenAPI.Description,
			ServerURL:   ctx.Config.Rest.OpenAPI.ServerURL,
			UIPath:      ctx.Config.Rest.OpenAPI.UIPath,
			SpecPath:    ctx.Config.Rest.OpenAPI.SpecPath,
		},
		Build: generator.BuildFeatures{HTTPPort: ctx.Config.Rest.HTTP.Port},
		Metrics: generator.MetricsFeatures{
			Enabled: ctx.Config.Rest.Observability.Metrics.Enabled.Bool(),
			Path:    ctx.Config.Rest.Observability.Metrics.Path,
		},
		Mongo: generator.MongoFeatures{Models: models},
	}
}

func mongoMainSource(ctx Context, models []generator.MongoModel, swagger string) string {
	var routes strings.Builder
	for _, model := range models {
		if model.Embedded || model.Collection == "" {
			continue
		}
		base := routePath(ctx.Config.Rest.HTTP.BasePath, "/"+strings.Trim(model.Collection, "/"))
		fmt.Fprintf(&routes, "\trouter.HandleFunc(%q, listDocuments(database.Collection(%q))).Methods(http.MethodGet)\n", base, model.Collection)
		fmt.Fprintf(&routes, "\trouter.HandleFunc(%q, createDocument(database.Collection(%q))).Methods(http.MethodPost)\n", base, model.Collection)
		fmt.Fprintf(&routes, "\trouter.HandleFunc(%q, getDocument(database.Collection(%q))).Methods(http.MethodGet)\n", base+"/{id}", model.Collection)
		fmt.Fprintf(&routes, "\trouter.HandleFunc(%q, updateDocument(database.Collection(%q))).Methods(http.MethodPatch)\n", base+"/{id}", model.Collection)
		fmt.Fprintf(&routes, "\trouter.HandleFunc(%q, deleteDocument(database.Collection(%q))).Methods(http.MethodDelete)\n", base+"/{id}", model.Collection)
	}
	rootPath := routePath(ctx.Config.Rest.HTTP.BasePath, "/")
	healthPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.HTTP.Health.Path)
	specPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.SpecPath)
	uiPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.UIPath)
	return fmt.Sprintf(`package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const swaggerSpec = %s

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	timeout, err := time.ParseDuration(%q)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(env("MONGO_URI", "mongodb://localhost:27017")))
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	}()
	database := client.Database(%q)
	router := mux.NewRouter()
	router.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}).Methods(http.MethodGet)
	router.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}).Methods(http.MethodGet)
	router.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(swaggerSpec))
	}).Methods(http.MethodGet)
	router.HandleFunc(%q, swaggerUI).Methods(http.MethodGet)
%s
	server := &http.Server{
		Addr:              env("HTTP_ADDR", %q),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %%s", server.Addr)
	return server.ListenAndServe()
}

func listDocuments(collection *mongo.Collection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		defer cursor.Close(ctx)
		var items []bson.M
		if err := cursor.All(ctx, &items); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if items == nil {
			items = []bson.M{}
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func createDocument(collection *mongo.Collection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input bson.M
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		delete(input, "id")
		delete(input, "_id")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, err := collection.InsertOne(ctx, input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		input["_id"] = result.InsertedID
		writeJSON(w, http.StatusOK, input)
	}
}

func getDocument(collection *mongo.Collection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := objectID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		var item bson.M
		if err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&item); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, mongo.ErrNoDocuments) {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func updateDocument(collection *mongo.Collection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := objectID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var input bson.M
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		delete(input, "id")
		delete(input, "_id")
		if len(input) == 0 {
			writeError(w, http.StatusBadRequest, errors.New("empty update body"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		var item bson.M
		err = collection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": input}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&item)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, mongo.ErrNoDocuments) {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func deleteDocument(collection *mongo.Collection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := objectID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if _, err := collection.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	}
}

func objectID(r *http.Request) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(mux.Vars(r)["id"])
}

func swaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`+"`"+`<!doctype html><html><head><title>Generated REST API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:%q,dom_id:'#swagger-ui'});</script></body></html>`+"`"+`))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
`, strconv.Quote(swagger), mongoTimeout(ctx), ctx.Config.Mongo.Connection.Database, rootPath, healthPath, specPath, uiPath, routes.String(), mongoHTTPAddr(ctx), specPath)
}

func mongoHTTPAddr(ctx Context) string {
	host := ctx.Config.Rest.HTTP.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := ctx.Config.Rest.HTTP.Port
	if port == 0 {
		port = 8080
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func mongoTimeout(ctx Context) string {
	timeout := ctx.Config.Mongo.Connection.Timeout
	if timeout == "" {
		timeout = "10s"
	}
	return timeout
}
