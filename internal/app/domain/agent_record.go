package domain

import (
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/repomz/rest_generator/internal/app/db"
)

type AgentRecord struct {
	ID      uuid.UUID
	AgentID int32
	Status  string
	SentAt  time.Time
}

type AgentRecordDB struct {
	AgentID int32
	Status  string
}

type AgentRecordDBData struct {
	AgentID int32
	Status  string
}

func NewAgentRecordDB(data AgentRecordDBData) (AgentRecordDB, error) {
	item := AgentRecordDB{
		AgentID: data.AgentID,
		Status:  data.Status,
	}
	if err := item.Validate(); err != nil {
		return AgentRecordDB{}, err
	}
	return item, nil
}

func (item AgentRecordDB) Validate() error {
	if !(item.AgentID != 0) {
		return fmt.Errorf("%w: agent_id", ErrRequired)
	}
	if !(item.Status != "") {
		return fmt.Errorf("%w: status", ErrRequired)
	}
	return nil
}

type CreateAgentRecordParams struct {
	AgentID int32
	Status  string
}

func (params CreateAgentRecordParams) Validate() error {
	if !(params.AgentID != 0) {
		return fmt.Errorf("%w: agent_id", ErrRequired)
	}
	if !(params.Status != "") {
		return fmt.Errorf("%w: status", ErrRequired)
	}
	return nil
}

type DeleteAgentRecordsByAgentIDParams struct {
	AgentID int32
}

func (params DeleteAgentRecordsByAgentIDParams) Validate() error {
	if !(params.AgentID != 0) {
		return fmt.Errorf("%w: agent_id", ErrRequired)
	}
	return nil
}

type GetAgentRecordsByAgentIDParams struct {
	AgentID int32
}

func (params GetAgentRecordsByAgentIDParams) Validate() error {
	if !(params.AgentID != 0) {
		return fmt.Errorf("%w: agent_id", ErrRequired)
	}
	return nil
}

type GetAgentRecordsByAgentIDandStatusParams struct {
	AgentID int32
	Status  string
}

func (params GetAgentRecordsByAgentIDandStatusParams) Validate() error {
	if !(params.AgentID != 0) {
		return fmt.Errorf("%w: agent_id", ErrRequired)
	}
	if !(params.Status != "") {
		return fmt.Errorf("%w: status", ErrRequired)
	}
	return nil
}

type GetAgentRecordsByStatusParams struct {
	Status string
}

func (params GetAgentRecordsByStatusParams) Validate() error {
	if !(params.Status != "") {
		return fmt.Errorf("%w: status", ErrRequired)
	}
	return nil
}

func NewAgentRecordFromDB(item db.AgentRecord) AgentRecord {
	return AgentRecord{
		ID:      item.ID,
		AgentID: item.AgentID,
		Status:  item.Status,
		SentAt:  item.SentAt,
	}
}
