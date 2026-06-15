package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/repomz/rest_generator/internal/app/common/server"
	"github.com/repomz/rest_generator/internal/app/domain"
	"github.com/repomz/rest_generator/internal/app/transport/httpmodels"
)

func (h HttpServer) CreateAgentRecord(w http.ResponseWriter, r *http.Request) {
	var params domain.CreateAgentRecordParams
	var body httpmodels.CreateAgentRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	params.AgentID = body.AgentID
	params.Status = body.Status

	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	if err := h.agent_recordService.CreateAgentRecord(r.Context(), params); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"ok": true}, w, r)
}

func (h HttpServer) DeleteAgentRecordsByAgentID(w http.ResponseWriter, r *http.Request) {
	var params domain.DeleteAgentRecordsByAgentIDParams
	rawAgentID := mux.Vars(r)["agent_id"]
	if rawAgentID == "" {
		server.BadRequest("missing-agent_id", domain.ErrRequired, w, r)
		return
	}
	value, err := strconv.Atoi(rawAgentID)
	if err != nil {
		server.BadRequest("invalid-agent_id", err, w, r)
		return
	}
	params.AgentID = int32(value)
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	if err := h.agent_recordService.DeleteAgentRecordsByAgentID(r.Context(), params); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"ok": true}, w, r)
}

func (h HttpServer) GetAgentRecordsByAgentID(w http.ResponseWriter, r *http.Request) {
	var params domain.GetAgentRecordsByAgentIDParams
	rawAgentID := mux.Vars(r)["agent_id"]
	if rawAgentID == "" {
		server.BadRequest("missing-agent_id", domain.ErrRequired, w, r)
		return
	}
	value, err := strconv.Atoi(rawAgentID)
	if err != nil {
		server.BadRequest("invalid-agent_id", err, w, r)
		return
	}
	params.AgentID = int32(value)
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.agent_recordService.GetAgentRecordsByAgentID(r.Context(), params)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(items, w, r)
}

func (h HttpServer) GetAgentRecordsByAgentIDandStatus(w http.ResponseWriter, r *http.Request) {
	var params domain.GetAgentRecordsByAgentIDandStatusParams
	rawAgentID := mux.Vars(r)["agent_id"]
	if rawAgentID == "" {
		server.BadRequest("missing-agent_id", domain.ErrRequired, w, r)
		return
	}
	value, err := strconv.Atoi(rawAgentID)
	if err != nil {
		server.BadRequest("invalid-agent_id", err, w, r)
		return
	}
	params.AgentID = int32(value)
	rawStatus := mux.Vars(r)["status"]
	if rawStatus == "" {
		server.BadRequest("missing-status", domain.ErrRequired, w, r)
		return
	}
	params.Status = rawStatus
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.agent_recordService.GetAgentRecordsByAgentIDandStatus(r.Context(), params)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(items, w, r)
}

func (h HttpServer) GetAgentRecordsByStatus(w http.ResponseWriter, r *http.Request) {
	var params domain.GetAgentRecordsByStatusParams
	rawStatus := mux.Vars(r)["status"]
	if rawStatus == "" {
		server.BadRequest("missing-status", domain.ErrRequired, w, r)
		return
	}
	params.Status = rawStatus
	if err := params.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	items, err := h.agent_recordService.GetAgentRecordsByStatus(r.Context(), params)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(items, w, r)
}
