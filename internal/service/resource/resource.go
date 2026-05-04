package resource

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Repository interface {
	CreateResource(ctx context.Context, resource *models.Resource) (*models.Resource, error)
	GetResource(ctx context.Context, resourceID string) (*models.Resource, error)
	GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error)
	GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
	UpdateResource(ctx context.Context, resourceID string, in UpdateResourceRequest) (*models.Resource, error)
	DeleteResource(ctx context.Context, resourceID string) error
	ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error)
}
type Service struct {
	log    *slog.Logger
	tracer trace.Tracer
	repo   Repository
}

func NewService(repo Repository, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}

	return &Service{
		repo:   repo,
		log:    log,
		tracer: otel.GetTracerProvider().Tracer("resource/service"),
	}
}

func (s *Service) CreateResource(ctx context.Context, in CreateResourceRequest) (*models.Resource, error) {
	const op = "Service.CreateResource"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resource := &models.Resource{
		Name:     in.Name,
		Type:     in.Type,
		Location: in.Location,
		Details:  in.Details,
		Status:   models.ResourceStatusAvailable,
	}

	createdResource, err := s.repo.CreateResource(ctx, resource)
	if err != nil {
		s.log.ErrorContext(ctx, "create resource failed", slog.String("type", string(in.Type)), slog.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s.log.InfoContext(ctx, "resource created", slog.String("resource_id", createdResource.ID), slog.String("type", string(createdResource.Type)))

	return createdResource, nil
}

func (s *Service) GetResource(ctx context.Context, resourceID string) (*models.Resource, error) {
	const op = "Service.GetResource"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resource, nil
}

func (s *Service) GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error) {
	const op = "Service.GetResourcesList"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resources, err := s.repo.GetResourcesList(ctx, types)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resources, nil
}

func (s *Service) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	const op = "Service.GetAvailableResources"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resources, err := s.repo.GetAvailableResources(ctx, types, location)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resources, nil
}

func (s *Service) UpdateResource(ctx context.Context, resourceID string, in UpdateResourceRequest) (*models.Resource, error) {
	const op = "Service.UpdateResource"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	repoReq := UpdateResourceRequest{
		Name:     in.Name,
		Location: in.Location,
		Details:  in.Details,
	}

	updatedResource, err := s.repo.UpdateResource(ctx, resourceID, repoReq)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			s.log.WarnContext(ctx, "update resource not found", slog.String("resource_id", resourceID))
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		if errors.Is(err, models.ErrInvalidType) {
			s.log.WarnContext(ctx, "update resource failed: invalid details type", slog.String("resource_id", resourceID))
			return nil, fmt.Errorf("%s: %w", op, models.ErrInvalidType)
		}

		s.log.ErrorContext(ctx, "update resource failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s.log.InfoContext(ctx, "resource updated", slog.String("resource_id", resourceID))

	return updatedResource, nil
}

func (s *Service) DeleteResource(ctx context.Context, resourceID string) error {
	const op = "Service.DeleteResource"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	if err := s.repo.DeleteResource(ctx, resourceID); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			s.log.WarnContext(ctx, "delete resource not found", slog.String("resource_id", resourceID))
		} else {
			s.log.ErrorContext(ctx, "delete resource failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	s.log.InfoContext(ctx, "resource deleted", slog.String("resource_id", resourceID))

	return nil
}

func (s *Service) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	const op = "Service.ChangeResourceStatus"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resource, err := s.repo.ChangeResourceStatus(ctx, resourceID, status, reason)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			s.log.WarnContext(ctx, "change status failed: resource not found", slog.String("resource_id", resourceID), slog.String("status", string(status)))
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		if errors.Is(err, models.ErrInvalidStatus) {
			s.log.WarnContext(ctx, "change status failed: invalid status", slog.String("resource_id", resourceID), slog.String("status", string(status)))
			return nil, fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
		}

		s.log.ErrorContext(ctx, "change status failed", slog.String("resource_id", resourceID), slog.String("status", string(status)), slog.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s.log.InfoContext(ctx, "resource status changed", slog.String("resource_id", resourceID), slog.String("status", string(resource.Status)))

	return resource, nil
}

func (s *Service) CheckResourceStatus(ctx context.Context, resourceID string) (bool, models.ResourceStatus, error) {
	const op = "Service.CheckResourceStatus"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			s.log.WarnContext(ctx, "check status failed: resource not found", slog.String("resource_id", resourceID))
			return false, "", fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}

		s.log.ErrorContext(ctx, "check status failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return false, "", fmt.Errorf("%s: %w", op, err)
	}

	isAvailable := resource.Status == models.ResourceStatusAvailable

	return isAvailable, resource.Status, nil
}

func (s *Service) UpdateResourceOccupancy(ctx context.Context, resourceID string, isOccupied bool) (models.ResourceStatus, error) {
	const op = "Service.UpdateResourceOccupancy"

	ctx, span := s.tracer.Start(ctx, op)
	defer span.End()

	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			s.log.WarnContext(ctx, "update occupancy failed: resource not found", slog.String("resource_id", resourceID))
			return "", fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}

		s.log.ErrorContext(ctx, "update occupancy failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if resource.Status == models.ResourceStatusMaintenance || resource.Status == models.ResourceStatusEmergency {
		s.log.WarnContext(ctx, "update occupancy rejected by status", slog.String("resource_id", resourceID), slog.String("status", string(resource.Status)))
		return "", fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
	}

	newStatus := models.ResourceStatusAvailable
	if isOccupied {
		newStatus = models.ResourceStatusOccupied
	}

	if resource.Status == newStatus {
		s.log.InfoContext(ctx, "resource occupancy unchanged", slog.String("resource_id", resourceID), slog.String("status", string(newStatus)))
		return newStatus, nil
	}

	updatedResource, err := s.repo.ChangeResourceStatus(ctx, resourceID, newStatus, "occupancy updated by booking service")
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			s.log.WarnContext(ctx, "update occupancy failed on status change: resource not found", slog.String("resource_id", resourceID))
			return "", fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		if errors.Is(err, models.ErrInvalidStatus) {
			s.log.WarnContext(ctx, "update occupancy failed on status change: invalid status", slog.String("resource_id", resourceID), slog.String("status", string(newStatus)))
			return "", fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
		}

		s.log.ErrorContext(ctx, "update occupancy failed on status change", slog.String("resource_id", resourceID), slog.String("status", string(newStatus)), slog.Any("error", err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	s.log.InfoContext(ctx, "resource occupancy updated", slog.String("resource_id", resourceID), slog.String("status", string(updatedResource.Status)))

	return updatedResource.Status, nil
}
