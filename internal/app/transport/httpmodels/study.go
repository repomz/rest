package httpmodels

import (
	"time"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type StudyRequest struct {
	StudyID        string    `json:"study_id"`
	Patient        string    `json:"patient"`
	Age            int32     `json:"age"`
	Department     string    `json:"department"`
	NameOperation  string    `json:"name_operation"`
	StudyType      string    `json:"study_type"`
	DescrOperation string    `json:"descr_operation"`
	TimeBeginning  time.Time `json:"time_beginning"`
	TimeDuration   int32     `json:"time_duration"`
	Surgeon        string    `json:"surgeon"`
	DicomLink      string    `json:"dicom_link"`
}

func (r StudyRequest) ToDomainDB() (domain.StudyDB, error) {
	return domain.NewStudyDB(domain.StudyDBData{
		StudyID:        r.StudyID,
		Patient:        r.Patient,
		Age:            r.Age,
		Department:     r.Department,
		NameOperation:  r.NameOperation,
		StudyType:      r.StudyType,
		DescrOperation: r.DescrOperation,
		TimeBeginning:  r.TimeBeginning,
		TimeDuration:   r.TimeDuration,
		Surgeon:        r.Surgeon,
		DicomLink:      r.DicomLink,
	})
}

type StudyResponse struct {
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StudyID        string    `json:"study_id"`
	Patient        string    `json:"patient"`
	Age            int32     `json:"age"`
	Department     string    `json:"department"`
	NameOperation  string    `json:"name_operation"`
	StudyType      string    `json:"study_type"`
	DescrOperation string    `json:"descr_operation"`
	TimeBeginning  time.Time `json:"time_beginning"`
	TimeDuration   int32     `json:"time_duration"`
	Surgeon        string    `json:"surgeon"`
	DicomLink      string    `json:"dicom_link"`
	Deleted        bool      `json:"deleted"`
}

func NewStudyResponse(item domain.Study) StudyResponse {
	return StudyResponse{
		ID:             item.ID,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		StudyID:        item.StudyID,
		Patient:        item.Patient,
		Age:            item.Age,
		Department:     item.Department,
		NameOperation:  item.NameOperation,
		StudyType:      item.StudyType,
		DescrOperation: item.DescrOperation,
		TimeBeginning:  item.TimeBeginning,
		TimeDuration:   item.TimeDuration,
		Surgeon:        item.Surgeon,
		DicomLink:      item.DicomLink,
		Deleted:        item.Deleted,
	}
}

type UpdateStudyDicomLinkRequest struct {
	DicomLink string `json:"dicom_link"`
}
