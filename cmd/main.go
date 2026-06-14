package main

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

	"github.com/repomz/rest_generator/internal/app/config"
	"github.com/repomz/rest_generator/internal/app/db"
	"github.com/repomz/rest_generator/internal/app/repository/pgrepo"
	"github.com/repomz/rest_generator/internal/app/services"
	"github.com/repomz/rest_generator/internal/app/transport/httpserver"
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
	studyRepo := pgrepo.NewStudyRepo(queries)
	studyService := services.NewStudyService(studyRepo)
	httpServer := httpserver.NewHttpServer(
		studyService,
	)

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Generated API"))
	}).Methods(http.MethodGet)
	router.HandleFunc("/studies", httpServer.CreateStudy).Methods(http.MethodPost)
	router.HandleFunc("/studies", httpServer.DeleteAllStudies).Methods(http.MethodDelete)
	router.HandleFunc("/studies", httpServer.GetStudies).Methods("GET")
	router.HandleFunc("/studies/patient/{patient}", httpServer.GetStudyByPatient).Methods("GET")
	router.HandleFunc("/studies/{id}/dicom-link", httpServer.UpdateStudyDicomLink).Methods("PATCH")
	router.HandleFunc("/studies/{id}", httpServer.GetStudyByID).Methods(http.MethodGet)
	router.HandleFunc("/studies/{id}", httpServer.DeleteStudy).Methods(http.MethodDelete)

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
