package pgrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/db"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type AgentRecordRepo struct {
	query *db.Queries
}

func NewAgentRecordRepo(query *db.Queries) *AgentRecordRepo {
	return &AgentRecordRepo{query: query}
}

func (r AgentRecordRepo) CreateAgentRecord(ctx context.Context, params domain.CreateAgentRecordParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	if err := r.query.CreateAgentRecord(ctx, db.CreateAgentRecordParams{
		AgentID: params.AgentID,
		Status:  params.Status,
	}); err != nil {
		return fmt.Errorf("failed to execute CreateAgentRecord: %w", err)
	}
	return nil
}

func (r AgentRecordRepo) DeleteAgentRecordsByAgentID(ctx context.Context, params domain.DeleteAgentRecordsByAgentIDParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	if err := r.query.DeleteAgentRecordsByAgentID(ctx, params.AgentID); err != nil {
		return fmt.Errorf("failed to execute DeleteAgentRecordsByAgentID: %w", err)
	}
	return nil
}

func (r AgentRecordRepo) GetAgentRecordsByAgentID(ctx context.Context, params domain.GetAgentRecordsByAgentIDParams) ([]time.Time, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	items, err := r.query.GetAgentRecordsByAgentID(ctx, params.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GetAgentRecordsByAgentID: %w", err)
	}
	return items, nil
}

func (r AgentRecordRepo) GetAgentRecordsByAgentIDandStatus(ctx context.Context, params domain.GetAgentRecordsByAgentIDandStatusParams) ([]time.Time, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	items, err := r.query.GetAgentRecordsByAgentIDandStatus(ctx, db.GetAgentRecordsByAgentIDandStatusParams{
		AgentID: params.AgentID,
		Status:  params.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute GetAgentRecordsByAgentIDandStatus: %w", err)
	}
	return items, nil
}

func (r AgentRecordRepo) GetAgentRecordsByStatus(ctx context.Context, params domain.GetAgentRecordsByStatusParams) ([]uuid.UUID, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	items, err := r.query.GetAgentRecordsByStatus(ctx, params.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GetAgentRecordsByStatus: %w", err)
	}
	return items, nil
}
