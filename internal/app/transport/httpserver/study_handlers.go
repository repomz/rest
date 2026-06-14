package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/rest_generator/internal/app/common/server"
	"github.com/repomz/rest_generator/internal/app/domain"
	"github.com/repomz/rest_generator/internal/app/transport/httpmodels"
)

func (h HttpServer) GetStudyByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	item, err := h.studyService.GetStudyByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(httpmodels.NewStudyResponse(item), w, r)
}

func (h HttpServer) CreateStudy(w http.ResponseWriter, r *http.Request) {
	var request httpmodels.StudyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	input, err := request.ToDomainDB()
	if err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	item, err := h.studyService.CreateStudy(r.Context(), input)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(httpmodels.NewStudyResponse(item), w, r)
}

func (h HttpServer) DeleteStudy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	if err := h.studyService.DeleteStudy(r.Context(), id); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

func (h HttpServer) DeleteAllStudies(w http.ResponseWriter, r *http.Request) {
	if err := h.studyService.DeleteAllStudies(r.Context()); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

func (h HttpServer) GetStudies(w http.ResponseWriter, r *http.Request) {
	var params domain.GetStudiesParams
	rawDate := r.URL.Query().Get("date")
	if rawDate != "" {
		value, err := time.Parse("2006-01-02", rawDate)
		if err != nil {
			server.BadRequest("invalid-date", err, w, r)
			return
		}
		params.Date = value
	}
	rawType := r.URL.Query().Get("type")
	if rawType != "" {
		params.Type = rawType
	}
	rawSurgeon := r.URL.Query().Get("surgeon")
	if rawSurgeon != "" {
		params.Surgeon = rawSurgeon
	}
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.studyService.GetStudies(r.Context(), params)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	response := make([]httpmodels.StudyResponse, 0, len(items))
	for _, item := range items {
		response = append(response, httpmodels.NewStudyResponse(item))
	}
	server.RespondOK(response, w, r)
}

func (h HttpServer) GetStudyByPatient(w http.ResponseWriter, r *http.Request) {
	var params domain.GetStudyByPatientParams
	rawPatient := mux.Vars(r)["patient"]
	if rawPatient == "" {
		server.BadRequest("missing-patient", domain.ErrRequired, w, r)
		return
	}
	params.Patient = rawPatient
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.studyService.GetStudyByPatient(r.Context(), params)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(httpmodels.NewStudyResponse(items), w, r)
}

func (h HttpServer) UpdateStudyDicomLink(w http.ResponseWriter, r *http.Request) {
	var params domain.UpdateStudyDicomLinkParams
	var body httpmodels.UpdateStudyDicomLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	params.DicomLink = body.DicomLink
	rawID := mux.Vars(r)["id"]
	if rawID == "" {
		server.BadRequest("missing-id", domain.ErrRequired, w, r)
		return
	}
	value, err := uuid.Parse(rawID)
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	params.ID = value
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.studyService.UpdateStudyDicomLink(r.Context(), params)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(httpmodels.NewStudyResponse(items), w, r)
}
