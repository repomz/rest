package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type fakeAgentRecordService struct{}

func sampleAgentRecord() domain.AgentRecord {
	return domain.AgentRecord{
		ID:      uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		AgentID: 1,
		Status:  "test_status",
		SentAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func (fakeAgentRecordService) CreateAgentRecord(ctx context.Context, params domain.CreateAgentRecordParams) error {
	return nil
}

func (fakeAgentRecordService) DeleteAgentRecordsByAgentID(ctx context.Context, params domain.DeleteAgentRecordsByAgentIDParams) error {
	return nil
}

func (fakeAgentRecordService) GetAgentRecordsByAgentID(ctx context.Context, params domain.GetAgentRecordsByAgentIDParams) ([]time.Time, error) {
	return []time.Time{time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}, nil
}

func (fakeAgentRecordService) GetAgentRecordsByAgentIDandStatus(ctx context.Context, params domain.GetAgentRecordsByAgentIDandStatusParams) ([]time.Time, error) {
	return []time.Time{time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}, nil
}

func (fakeAgentRecordService) GetAgentRecordsByStatus(ctx context.Context, params domain.GetAgentRecordsByStatusParams) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.MustParse("00000000-0000-0000-0000-000000000001")}, nil
}

func testAgentRecordHandlersRouter() *mux.Router {
	httpServer := NewHttpServer(
		fakeAgentRecordService{},
		fakeStudyService{},
	)
	router := mux.NewRouter()
	router.HandleFunc("/agent_records", httpServer.CreateAgentRecord).Methods("POST")
	router.HandleFunc("/agent_records/agent/{agent_id}", httpServer.DeleteAgentRecordsByAgentID).Methods("DELETE")
	router.HandleFunc("/agent_records/agent/{agent_id}", httpServer.GetAgentRecordsByAgentID).Methods("GET")
	router.HandleFunc("/agent_records/agent/{agent_id}/status/{status}", httpServer.GetAgentRecordsByAgentIDandStatus).Methods("GET")
	router.HandleFunc("/agent_records/status/{status}", httpServer.GetAgentRecordsByStatus).Methods("GET")
	return router
}

func TestAgentRecordHandlers(t *testing.T) {
	router := testAgentRecordHandlersRouter()
	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "CreateAgentRecord", method: "POST", url: "/agent_records", body: `{"agent_id":1,"status":"test_status"}`},
		{name: "DeleteAgentRecordsByAgentID", method: "DELETE", url: "/agent_records/agent/1", body: ``},
		{name: "GetAgentRecordsByAgentID", method: "GET", url: "/agent_records/agent/1", body: ``},
		{name: "GetAgentRecordsByAgentIDandStatus", method: "GET", url: "/agent_records/agent/1/status/test_status", body: ``},
		{name: "GetAgentRecordsByStatus", method: "GET", url: "/agent_records/status/test_status", body: ``},
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
