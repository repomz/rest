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

type fakeStudyService struct{}

func sampleStudy() domain.Study {
	return domain.Study{
		ID:             uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		CreatedAt:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		StudyID:        "test_study_id",
		Patient:        "test_patient",
		Age:            1,
		Department:     "test_department",
		NameOperation:  "test_name_operation",
		StudyType:      "test_study_type",
		DescrOperation: "test_descr_operation",
		TimeBeginning:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		TimeDuration:   1,
		Surgeon:        "test_surgeon",
		DicomLink:      "test_dicom_link",
		Deleted:        false,
	}
}

func (fakeStudyService) GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error) {
	return sampleStudy(), nil
}

func (fakeStudyService) CreateStudy(ctx context.Context, item domain.StudyDB) (domain.Study, error) {
	return sampleStudy(), nil
}

func (fakeStudyService) DeleteStudy(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (fakeStudyService) DeleteAllStudies(ctx context.Context) error {
	return nil
}

func (fakeStudyService) GetStudies(ctx context.Context, params domain.GetStudiesParams) ([]domain.Study, error) {
	return []domain.Study{sampleStudy()}, nil
}

func (fakeStudyService) GetStudyByPatient(ctx context.Context, params domain.GetStudyByPatientParams) (domain.Study, error) {
	return sampleStudy(), nil
}

func (fakeStudyService) UpdateStudyDicomLink(ctx context.Context, params domain.UpdateStudyDicomLinkParams) (domain.Study, error) {
	return sampleStudy(), nil
}

func testStudyHandlersRouter() *mux.Router {
	httpServer := NewHttpServer(
		fakeAgentRecordService{},
		fakeStudyService{},
	)
	router := mux.NewRouter()
	router.HandleFunc("/studies", httpServer.CreateStudy).Methods(http.MethodPost)
	router.HandleFunc("/studies", httpServer.DeleteAllStudies).Methods(http.MethodDelete)
	router.HandleFunc("/studies", httpServer.GetStudies).Methods("GET")
	router.HandleFunc("/studies/patient/{patient}", httpServer.GetStudyByPatient).Methods("GET")
	router.HandleFunc("/studies/{id}/dicom-link", httpServer.UpdateStudyDicomLink).Methods("PATCH")
	router.HandleFunc("/studies/{id}", httpServer.GetStudyByID).Methods(http.MethodGet)
	router.HandleFunc("/studies/{id}", httpServer.DeleteStudy).Methods(http.MethodDelete)
	return router
}

func TestStudyHandlers(t *testing.T) {
	router := testStudyHandlersRouter()
	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "create study", method: http.MethodPost, url: "/studies", body: `{"study_id":"test_study_id","patient":"test_patient","age":1,"department":"test_department","name_operation":"test_name_operation","study_type":"test_study_type","descr_operation":"test_descr_operation","time_beginning":"2026-01-02T00:00:00Z","time_duration":1,"surgeon":"test_surgeon","dicom_link":"test_dicom_link"}`},
		{name: "delete all studies", method: http.MethodDelete, url: "/studies"},
		{name: "GetStudies", method: "GET", url: "/studies?date=2026-01-02&type=test_type&surgeon=test_surgeon", body: ``},
		{name: "GetStudyByPatient", method: "GET", url: "/studies/patient/test_patient", body: ``},
		{name: "UpdateStudyDicomLink", method: "PATCH", url: "/studies/00000000-0000-0000-0000-000000000001/dicom-link", body: `{"dicom_link":"test_dicom_link"}`},
		{name: "get study by id", method: http.MethodGet, url: "/studies/00000000-0000-0000-0000-000000000001"},
		{name: "delete study", method: http.MethodDelete, url: "/studies/00000000-0000-0000-0000-000000000001"},
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
