package resource

import (
	"context"
	"errors"
	"fmt"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
)

type Repository interface {
	CreateResource(ctx context.Context, resource *models.Resource) (*models.Resource, error)
	GetResource(ctx context.Context, resourceID string) (*models.Resource, error)
	GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error)
	GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
	UpdateResource(ctx context.Context, resourceID string, in UpdateResourceRequest, fields []string) (*models.Resource, error)
	DeleteResource(ctx context.Context, resourceID string) error
	ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error)
}
type Service struct {
	// log *slog.Logger
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateResource(ctx context.Context, in CreateResourceRequest) (*models.Resource, error) {
	const op = "Service.CreateResource"

	resource := &models.Resource{
		Name:     in.Name,
		Type:     in.Type,
		Location: in.Location,
		Details:  in.Details,
		Status:   models.ResourceStatusAvailable,
	}

	createdResource, err := s.repo.CreateResource(ctx, resource)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return createdResource, nil
}

func (s *Service) GetResource(ctx context.Context, resourceID string) (*models.Resource, error) {
	const op = "Service.GetResource"

	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resource, nil
}

func (s *Service) GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error) {
	const op = "Service.GetResourcesList"

	resources, err := s.repo.GetResourcesList(ctx, types)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resources, nil
}

func (s *Service) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	const op = "Service.GetAvailableResources"

	resources, err := s.repo.GetAvailableResources(ctx, types, location)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resources, nil
}

func (s *Service) UpdateResource(ctx context.Context, resourceID string, in UpdateResourceRequest, fields []string) (*models.Resource, error) {
	const op = "Service.UpdateResource"

	for _, field := range fields {
		switch field {
		case "name", "location", "details":
		default:
			return nil, fmt.Errorf("%s: unknown field %q", op, field)
		}
	}

	if containsField(fields, "details") {
		resource, err := s.repo.GetResource(ctx, resourceID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		if err := validateDetailsByType(resource.Type, in.Details); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}

	repoReq := UpdateResourceRequest{
		Name:     in.Name,
		Location: in.Location,
		Details:  in.Details,
	}

	updatedResource, err := s.repo.UpdateResource(ctx, resourceID, repoReq, fields)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return updatedResource, nil
}

func (s *Service) DeleteResource(ctx context.Context, resourceID string) error {
	const op = "Service.DeleteResource"

	if err := s.repo.DeleteResource(ctx, resourceID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Service) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	const op = "Service.ChangeResourceStatus"

	resource, err := s.repo.ChangeResourceStatus(ctx, resourceID, status, reason)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		if errors.Is(err, models.ErrInvalidStatus) {
			return nil, fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resource, nil
}

func (s *Service) CheckResourceStatus(ctx context.Context, resourceID string) (bool, models.ResourceStatus, error) {
	const op = "Service.CheckResourceStatus"

	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return false, "", fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		return false, "", fmt.Errorf("%s: %w", op, err)
	}

	isAvailable := resource.Status == models.ResourceStatusAvailable

	return isAvailable, resource.Status, nil
}

func (s *Service) UpdateResourceOccupancy(ctx context.Context, resourceID string, isOccupied bool) (models.ResourceStatus, error) {
	const op = "Service.UpdateResourceOccupancy"

	resource, err := s.repo.GetResource(ctx, resourceID)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return "", fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if resource.Status == models.ResourceStatusMaintenance || resource.Status == models.ResourceStatusEmergency {
		return "", fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
	}

	newStatus := models.ResourceStatusAvailable
	if isOccupied {
		newStatus = models.ResourceStatusOccupied
	}

	if resource.Status == newStatus {
		return newStatus, nil
	}

	updatedResource, err := s.repo.ChangeResourceStatus(ctx, resourceID, newStatus, "occupancy updated by booking service")
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return "", fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		if errors.Is(err, models.ErrInvalidStatus) {
			return "", fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return updatedResource.Status, nil
}

//////////////////////////////////////////////////////////////////////

func containsField(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}

	return false
}

func validateDetailsByType(resourceType models.ResourceType, details any) error {
	switch resourceType {
	case models.ResourceTypeMeetingRoom:
		if _, ok := details.(*models.MeetingRoomDetails); !ok {
			return fmt.Errorf("meeting_room details required for type MEETING_ROOM")
		}
	case models.ResourceTypeWorkspace:
		if _, ok := details.(*models.WorkspaceDetails); !ok {
			return fmt.Errorf("workspace details required for type WORKSPACE")
		}
	case models.ResourceTypeDevice:
		if _, ok := details.(*models.DeviceDetails); !ok {
			return fmt.Errorf("device details required for type DEVICE")
		}
	default:
		return fmt.Errorf("resource type is required")
	}

	return nil
}
