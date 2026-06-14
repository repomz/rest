package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type StudyRepository interface {
	GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error)
	CreateStudy(ctx context.Context, item domain.StudyDB) (domain.Study, error)
	DeleteStudy(ctx context.Context, id uuid.UUID) error
	DeleteAllStudies(ctx context.Context) error
	GetStudies(ctx context.Context, params domain.GetStudiesParams) ([]domain.Study, error)
	GetStudyByPatient(ctx context.Context, params domain.GetStudyByPatientParams) (domain.Study, error)
	UpdateStudyDicomLink(ctx context.Context, params domain.UpdateStudyDicomLinkParams) (domain.Study, error)
}

type StudyService struct {
	repo StudyRepository
}

func NewStudyService(repo StudyRepository) StudyService {
	return StudyService{repo: repo}
}

func (s StudyService) GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error) {
	return s.repo.GetStudyByID(ctx, id)
}

func (s StudyService) CreateStudy(ctx context.Context, item domain.StudyDB) (domain.Study, error) {
	return s.repo.CreateStudy(ctx, item)
}

func (s StudyService) DeleteStudy(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteStudy(ctx, id)
}

func (s StudyService) DeleteAllStudies(ctx context.Context) error {
	return s.repo.DeleteAllStudies(ctx)
}

func (s StudyService) GetStudies(ctx context.Context, params domain.GetStudiesParams) ([]domain.Study, error) {
	return s.repo.GetStudies(ctx, params)
}

func (s StudyService) GetStudyByPatient(ctx context.Context, params domain.GetStudyByPatientParams) (domain.Study, error) {
	return s.repo.GetStudyByPatient(ctx, params)
}

func (s StudyService) UpdateStudyDicomLink(ctx context.Context, params domain.UpdateStudyDicomLinkParams) (domain.Study, error) {
	return s.repo.UpdateStudyDicomLink(ctx, params)
}
