package booking

import (
	"context"
	"errors"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"github.com/acyushka/oregon-resource-service/internal/grpc/resource/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceServiceBooking interface {
	GetResource(ctx context.Context, resourceID string) (*models.Resource, error)
	CheckResourceStatus(ctx context.Context, resourceID string) (bool, models.ResourceStatus, error)
	GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
	UpdateResourceOccupancy(ctx context.Context, resourceID string, isOccupied bool) (models.ResourceStatus, error)
}

type ServerAPI struct {
	resourcev1.UnimplementedResourceBookingServiceServer
	service ResourceServiceBooking
}

func NewServer(gRPCServer *grpc.Server, service ResourceServiceBooking) {
	resourcev1.RegisterResourceBookingServiceServer(gRPCServer, &ServerAPI{service: service})
}

func (s *ServerAPI) GetResource(ctx context.Context, in *resourcev1.GetResourceRequest) (*resourcev1.GetResourceResponse, error) {
	if in.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource ID is required")
	}

	resource, err := s.service.GetResource(ctx, in.GetResourceId())
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to retrieve resource")
	}

	return &resourcev1.GetResourceResponse{Resource: utils.MapServiceResourceToProto(resource)}, nil
}

func (s *ServerAPI) CheckResourceStatus(ctx context.Context, in *resourcev1.CheckResourceStatusRequest) (*resourcev1.CheckResourceStatusResponse, error) {
	if in.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource ID is required")
	}

	isAvailable, resourceStatus, err := s.service.CheckResourceStatus(ctx, in.GetResourceId())
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to check resource status")
	}

	return &resourcev1.CheckResourceStatusResponse{
		IsAvailable: isAvailable,
		Status:      utils.ServiceStatusToProto(resourceStatus),
	}, nil
}

func (s *ServerAPI) GetAvailableResources(ctx context.Context, in *resourcev1.GetAvailableResourcesRequest) (*resourcev1.GetAvailableResourcesResponse, error) {
	types, err := utils.ProtoTypesToService(in.GetTypes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	resources, err := s.service.GetAvailableResources(ctx, types, in.GetLocation())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve available resources")
	}

	protoResources := make([]*resourcev1.Resource, len(resources))
	for i, res := range resources {
		protoResources[i] = utils.MapServiceResourceToProto(res)
	}

	return &resourcev1.GetAvailableResourcesResponse{
		Resources: protoResources,
		//nolint:gosec
		TotalCount: int32(len(protoResources)),
	}, nil
}

func (s *ServerAPI) UpdateResourceOccupancy(ctx context.Context, in *resourcev1.UpdateResourceOccupancyRequest) (*resourcev1.UpdateResourceOccupancyResponse, error) {
	if in.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource ID is required")
	}

	resStatus, err := s.service.UpdateResourceOccupancy(ctx, in.GetResourceId(), in.GetIsOccupied())
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, models.ErrInvalidStatus) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to update resource occupancy")
	}

	return &resourcev1.UpdateResourceOccupancyResponse{
		Success: true,
		Status:  utils.ServiceStatusToProto(resStatus),
	}, nil
}
