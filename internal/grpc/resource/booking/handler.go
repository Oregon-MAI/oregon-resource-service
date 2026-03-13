package booking

import (
	"context"
	"errors"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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

	return &resourcev1.GetResourceResponse{Resource: mapServiceResourceToProto(resource)}, nil
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
		Status:      serviceStatusToProto(resourceStatus),
	}, nil
}

func (s *ServerAPI) GetAvailableResources(ctx context.Context, in *resourcev1.GetAvailableResourcesRequest) (*resourcev1.GetAvailableResourcesResponse, error) {
	types := make([]models.ResourceType, 0, len(in.GetTypes()))
	for _, t := range in.GetTypes() {
		types = append(types, models.ResourceType(t))
	}

	resources, err := s.service.GetAvailableResources(ctx, types, in.GetLocation())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve available resources")
	}

	protoResources := make([]*resourcev1.Resource, len(resources))
	for i, res := range resources {
		protoResources[i] = mapServiceResourceToProto(res)
	}

	return &resourcev1.GetAvailableResourcesResponse{
		Resources:  protoResources,
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
		Status:  serviceStatusToProto(resStatus),
	}, nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////

func mapServiceResourceToProto(res *models.Resource) *resourcev1.Resource {
	proto := &resourcev1.Resource{
		ResourceId: res.ID,
		Name:       res.Name,
		Type:       serviceTypeToProto(res.Type),
		Location:   res.Location,
		Status:     serviceStatusToProto(res.Status),
		CreatedAt:  timestamppb.New(res.CreatedAt),
		UpdatedAt:  timestamppb.New(res.UpdatedAt),
	}

	switch details := res.Details.(type) {
	case *models.MeetingRoomDetails:
		proto.Details = &resourcev1.Resource_MeetingRoom{
			MeetingRoom: &resourcev1.MeetingRoomDetails{
				Capacity:      details.Capacity,
				HasProjector:  details.HasProjector,
				HasWhiteboard: details.HasWhiteboard,
			},
		}
	case *models.WorkspaceDetails:
		proto.Details = &resourcev1.Resource_Workspace{
			Workspace: &resourcev1.WorkspaceDetails{
				HasMonitor: details.HasMonitor,
			},
		}
	case *models.DeviceDetails:
		proto.Details = &resourcev1.Resource_Device{
			Device: &resourcev1.DeviceDetails{
				DeviceType:   details.DeviceType,
				Model:        details.Model,
				SerialNumber: details.SerialNumber,
				Description:  details.Description,
			},
		}
	}

	return proto
}

func serviceStatusToProto(s models.ResourceStatus) resourcev1.ResourceStatus {
	switch s {
	case models.ResourceStatusAvailable:
		return resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE
	case models.ResourceStatusOccupied:
		return resourcev1.ResourceStatus_RESOURCE_STATUS_OCCUPIED
	case models.ResourceStatusMaintenance:
		return resourcev1.ResourceStatus_RESOURCE_STATUS_MAINTENANCE
	case models.ResourceStatusEmergency:
		return resourcev1.ResourceStatus_RESOURCE_STATUS_EMERGENCY
	default:
		return resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED
	}
}

func serviceTypeToProto(t models.ResourceType) resourcev1.ResourceType {
	switch t {
	case models.ResourceTypeMeetingRoom:
		return resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM
	case models.ResourceTypeWorkspace:
		return resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE
	case models.ResourceTypeDevice:
		return resourcev1.ResourceType_RESOURCE_TYPE_DEVICE
	default:
		return resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED
	}
}
