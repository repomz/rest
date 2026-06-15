package httpmodels

import (
	"time"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type AgentRecordRequest struct {
	AgentID int32  `json:"agent_id"`
	Status  string `json:"status"`
}

func (r AgentRecordRequest) ToDomainDB() (domain.AgentRecordDB, error) {
	return domain.NewAgentRecordDB(domain.AgentRecordDBData{
		AgentID: r.AgentID,
		Status:  r.Status,
	})
}

type AgentRecordResponse struct {
	ID      uuid.UUID `json:"id"`
	AgentID int32     `json:"agent_id"`
	Status  string    `json:"status"`
	SentAt  time.Time `json:"sent_at"`
}

func NewAgentRecordResponse(item domain.AgentRecord) AgentRecordResponse {
	return AgentRecordResponse{
		ID:      item.ID,
		AgentID: item.AgentID,
		Status:  item.Status,
		SentAt:  item.SentAt,
	}
}

type CreateAgentRecordRequest struct {
	AgentID int32  `json:"agent_id"`
	Status  string `json:"status"`
}
