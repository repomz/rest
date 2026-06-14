package pgrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/db"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type StudyRepo struct {
	query *db.Queries
}

func NewStudyRepo(query *db.Queries) *StudyRepo {
	return &StudyRepo{query: query}
}

func (r StudyRepo) GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error) {
	item, err := r.query.GetStudyByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to get study: %w", err)
	}
	return domain.NewStudyFromDB(item), nil
}

func (r StudyRepo) CreateStudy(ctx context.Context, item domain.StudyDB) (domain.Study, error) {
	if err := item.Validate(); err != nil {
		return domain.Study{}, err
	}
	created, err := r.query.CreateStudy(ctx, db.CreateStudyParams{
		StudyID:        item.StudyID,
		Patient:        item.Patient,
		Age:            sql.NullInt32{Int32: item.Age, Valid: item.Age != 0},
		Department:     item.Department,
		NameOperation:  item.NameOperation,
		StudyType:      item.StudyType,
		DescrOperation: item.DescrOperation,
		TimeBeginning:  sql.NullTime{Time: item.TimeBeginning, Valid: !item.TimeBeginning.IsZero()},
		TimeDuration:   sql.NullInt32{Int32: item.TimeDuration, Valid: item.TimeDuration != 0},
		Surgeon:        item.Surgeon,
		DicomLink:      sql.NullString{String: item.DicomLink, Valid: item.DicomLink != ""},
	})
	if err != nil {
		return domain.Study{}, fmt.Errorf("failed to create study: %w", err)
	}
	return domain.NewStudyFromDB(created), nil
}

func (r StudyRepo) DeleteStudy(ctx context.Context, id uuid.UUID) error {
	if err := r.query.SoftDeleteStudy(ctx, id); err != nil {
		return fmt.Errorf("failed to delete study: %w", err)
	}
	return nil
}

func (r StudyRepo) DeleteAllStudies(ctx context.Context) error {
	if err := r.query.SoftDeleteAllStudies(ctx); err != nil {
		return fmt.Errorf("failed to delete studies: %w", err)
	}
	return nil
}

func (r StudyRepo) GetStudies(ctx context.Context, params domain.GetStudiesParams) ([]domain.Study, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	items, err := r.query.GetStudies(ctx, db.GetStudiesParams{
		Date:    sql.NullTime{Time: params.Date, Valid: !params.Date.IsZero()},
		Type:    sql.NullString{String: params.Type, Valid: params.Type != ""},
		Surgeon: sql.NullString{String: params.Surgeon, Valid: params.Surgeon != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute GetStudies: %w", err)
	}
	result := make([]domain.Study, 0, len(items))
	for _, item := range items {
		result = append(result, domain.NewStudyFromDB(item))
	}
	return result, nil
}

func (r StudyRepo) GetStudyByPatient(ctx context.Context, params domain.GetStudyByPatientParams) (domain.Study, error) {
	if err := params.Validate(); err != nil {
		return domain.Study{}, err
	}
	items, err := r.query.GetStudyByPatient(ctx, params.Patient)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to execute GetStudyByPatient: %w", err)
	}
	return domain.NewStudyFromDB(items), nil
}

func (r StudyRepo) UpdateStudyDicomLink(ctx context.Context, params domain.UpdateStudyDicomLinkParams) (domain.Study, error) {
	if err := params.Validate(); err != nil {
		return domain.Study{}, err
	}
	items, err := r.query.UpdateStudyDicomLink(ctx, db.UpdateStudyDicomLinkParams{
		ID:        params.ID,
		DicomLink: sql.NullString{String: params.DicomLink, Valid: params.DicomLink != ""},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Study{}, domain.ErrNotFound
		}
		return domain.Study{}, fmt.Errorf("failed to execute UpdateStudyDicomLink: %w", err)
	}
	return domain.NewStudyFromDB(items), nil
}
