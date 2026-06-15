package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/rest_generator/internal/app/domain"
)

type AgentRecordRepository interface {
	CreateAgentRecord(ctx context.Context, params domain.CreateAgentRecordParams) error
	DeleteAgentRecordsByAgentID(ctx context.Context, params domain.DeleteAgentRecordsByAgentIDParams) error
	GetAgentRecordsByAgentID(ctx context.Context, params domain.GetAgentRecordsByAgentIDParams) ([]time.Time, error)
	GetAgentRecordsByAgentIDandStatus(ctx context.Context, params domain.GetAgentRecordsByAgentIDandStatusParams) ([]time.Time, error)
	GetAgentRecordsByStatus(ctx context.Context, params domain.GetAgentRecordsByStatusParams) ([]uuid.UUID, error)
}

type AgentRecordService struct {
	repo AgentRecordRepository
}

func NewAgentRecordService(repo AgentRecordRepository) AgentRecordService {
	return AgentRecordService{repo: repo}
}

func (s AgentRecordService) CreateAgentRecord(ctx context.Context, params domain.CreateAgentRecordParams) error {
	return s.repo.CreateAgentRecord(ctx, params)
}

func (s AgentRecordService) DeleteAgentRecordsByAgentID(ctx context.Context, params domain.DeleteAgentRecordsByAgentIDParams) error {
	return s.repo.DeleteAgentRecordsByAgentID(ctx, params)
}

func (s AgentRecordService) GetAgentRecordsByAgentID(ctx context.Context, params domain.GetAgentRecordsByAgentIDParams) ([]time.Time, error) {
	return s.repo.GetAgentRecordsByAgentID(ctx, params)
}

func (s AgentRecordService) GetAgentRecordsByAgentIDandStatus(ctx context.Context, params domain.GetAgentRecordsByAgentIDandStatusParams) ([]time.Time, error) {
	return s.repo.GetAgentRecordsByAgentIDandStatus(ctx, params)
}

func (s AgentRecordService) GetAgentRecordsByStatus(ctx context.Context, params domain.GetAgentRecordsByStatusParams) ([]uuid.UUID, error) {
	return s.repo.GetAgentRecordsByStatus(ctx, params)
}
