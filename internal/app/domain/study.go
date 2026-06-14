package domain

import (
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/repomz/rest_generator/internal/app/db"
)

type Study struct {
	ID             uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudyID        string
	Patient        string
	Age            int32
	Department     string
	NameOperation  string
	StudyType      string
	DescrOperation string
	TimeBeginning  time.Time
	TimeDuration   int32
	Surgeon        string
	DicomLink      string
	Deleted        bool
}

type StudyDB struct {
	StudyID        string
	Patient        string
	Age            int32
	Department     string
	NameOperation  string
	StudyType      string
	DescrOperation string
	TimeBeginning  time.Time
	TimeDuration   int32
	Surgeon        string
	DicomLink      string
}

type StudyDBData struct {
	StudyID        string
	Patient        string
	Age            int32
	Department     string
	NameOperation  string
	StudyType      string
	DescrOperation string
	TimeBeginning  time.Time
	TimeDuration   int32
	Surgeon        string
	DicomLink      string
}

func NewStudyDB(data StudyDBData) (StudyDB, error) {
	item := StudyDB{
		StudyID:        data.StudyID,
		Patient:        data.Patient,
		Age:            data.Age,
		Department:     data.Department,
		NameOperation:  data.NameOperation,
		StudyType:      data.StudyType,
		DescrOperation: data.DescrOperation,
		TimeBeginning:  data.TimeBeginning,
		TimeDuration:   data.TimeDuration,
		Surgeon:        data.Surgeon,
		DicomLink:      data.DicomLink,
	}
	if err := item.Validate(); err != nil {
		return StudyDB{}, err
	}
	return item, nil
}

func (item StudyDB) Validate() error {
	if !(item.StudyID != "") {
		return fmt.Errorf("%w: study_id", ErrRequired)
	}
	if !(item.Patient != "") {
		return fmt.Errorf("%w: patient", ErrRequired)
	}
	if !(item.Department != "") {
		return fmt.Errorf("%w: department", ErrRequired)
	}
	if !(item.NameOperation != "") {
		return fmt.Errorf("%w: name_operation", ErrRequired)
	}
	if !(item.StudyType != "") {
		return fmt.Errorf("%w: study_type", ErrRequired)
	}
	if !(item.DescrOperation != "") {
		return fmt.Errorf("%w: descr_operation", ErrRequired)
	}
	if !(item.Surgeon != "") {
		return fmt.Errorf("%w: surgeon", ErrRequired)
	}
	return nil
}

type GetStudiesParams struct {
	Date    time.Time
	Type    string
	Surgeon string
}

func (params GetStudiesParams) Validate() error {
	return nil
}

type GetStudyByPatientParams struct {
	Patient string
}

func (params GetStudyByPatientParams) Validate() error {
	if !(params.Patient != "") {
		return fmt.Errorf("%w: patient", ErrRequired)
	}
	return nil
}

type UpdateStudyDicomLinkParams struct {
	ID        uuid.UUID
	DicomLink string
}

func (params UpdateStudyDicomLinkParams) Validate() error {
	if !(params.ID != uuid.Nil) {
		return fmt.Errorf("%w: id", ErrRequired)
	}
	if !(params.DicomLink != "") {
		return fmt.Errorf("%w: dicom_link", ErrRequired)
	}
	return nil
}

func NewStudyFromDB(item db.Study) Study {
	return Study{
		ID:             item.ID,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		StudyID:        item.StudyID,
		Patient:        item.Patient,
		Age:            item.Age.Int32,
		Department:     item.Department,
		NameOperation:  item.NameOperation,
		StudyType:      item.StudyType,
		DescrOperation: item.DescrOperation,
		TimeBeginning:  item.TimeBeginning.Time,
		TimeDuration:   item.TimeDuration.Int32,
		Surgeon:        item.Surgeon,
		DicomLink:      item.DicomLink.String,
		Deleted:        item.Deleted,
	}
}
