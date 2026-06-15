package httpserver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type AgentRecordService interface {
	CreateAgentRecord(ctx context.Context, params domain.CreateAgentRecordParams) error
	DeleteAgentRecordsByAgentID(ctx context.Context, params domain.DeleteAgentRecordsByAgentIDParams) error
	GetAgentRecordsByAgentID(ctx context.Context, params domain.GetAgentRecordsByAgentIDParams) ([]time.Time, error)
	GetAgentRecordsByAgentIDandStatus(ctx context.Context, params domain.GetAgentRecordsByAgentIDandStatusParams) ([]time.Time, error)
	GetAgentRecordsByStatus(ctx context.Context, params domain.GetAgentRecordsByStatusParams) ([]uuid.UUID, error)
}

type StudyService interface {
	GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error)
	CreateStudy(ctx context.Context, item domain.StudyDB) (domain.Study, error)
	DeleteStudy(ctx context.Context, id uuid.UUID) error
	DeleteAllStudies(ctx context.Context) error
	GetStudies(ctx context.Context, params domain.GetStudiesParams) ([]domain.Study, error)
	GetStudyByPatient(ctx context.Context, params domain.GetStudyByPatientParams) (domain.Study, error)
	UpdateStudyDicomLink(ctx context.Context, params domain.UpdateStudyDicomLinkParams) (domain.Study, error)
}
