package admin

import (
	"context"
	"errors"
	"fmt"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ResourceServiceAdmin interface {
	CreateResource(ctx context.Context, in CreateResourceRequest) (*models.Resource, error)
	GetResource(ctx context.Context, resourceID string) (*models.Resource, error)
	GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error)
	UpdateResource(ctx context.Context, resourceID string, in UpdateResourceRequest, fields []string) (*models.Resource, error)
	DeleteResource(ctx context.Context, resourceID string) error
	ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error)
}

type ServerAPI struct {
	resourcev1.UnimplementedResourceAdminServiceServer
	service ResourceServiceAdmin
}

func NewServer(gRPCServer *grpc.Server, service ResourceServiceAdmin) {
	resourcev1.RegisterResourceAdminServiceServer(gRPCServer, &ServerAPI{service: service})
}

func (s *ServerAPI) CreateResource(ctx context.Context, in *resourcev1.CreateResourceRequest) (*resourcev1.CreateResourceResponse, error) {
	if err := validateCreateResourceRequest(in); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	req := CreateResourceRequest{
		Name:     in.GetName(),
		Type:     models.ResourceType(in.GetType()),
		Location: in.GetLocation(),
		Details:  in.GetDetails(),
	}

	resource, err := s.service.CreateResource(ctx, req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &resourcev1.CreateResourceResponse{Resource: mapServiceResourceToProto(resource)}, nil
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

func (s *ServerAPI) GetResourcesList(ctx context.Context, in *resourcev1.GetResourcesListRequest) (*resourcev1.GetResourcesListResponse, error) {
	types := make([]models.ResourceType, 0, len(in.GetTypes()))
	for _, t := range in.GetTypes() {
		types = append(types, models.ResourceType(t))
	}

	resources, err := s.service.GetResourcesList(ctx, types)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve resources list")
	}

	protoResources := make([]*resourcev1.Resource, len(resources))
	for i, res := range resources {
		protoResources[i] = mapServiceResourceToProto(res)
	}

	return &resourcev1.GetResourcesListResponse{Resources: protoResources}, nil
}

func (s *ServerAPI) UpdateResource(ctx context.Context, in *resourcev1.UpdateResourceRequest) (*resourcev1.UpdateResourceResponse, error) {
	if in.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource ID is required")
	}
	if in.FieldMask == nil || len(in.GetFieldMask().GetPaths()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "field mask is required")
	}

	updateReq := buildUpdateResourceRequest(in)

	resource, err := s.service.UpdateResource(ctx, in.GetResourceId(), updateReq, in.GetFieldMask().GetPaths())
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to update resource")
	}

	return &resourcev1.UpdateResourceResponse{Resource: mapServiceResourceToProto(resource)}, nil
}

func (s *ServerAPI) DeleteResource(ctx context.Context, in *resourcev1.DeleteResourceRequest) (*resourcev1.DeleteResourceResponse, error) {
	if in.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource ID is required")
	}

	if err := s.service.DeleteResource(ctx, in.GetResourceId()); err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to delete resource")
	}

	return &resourcev1.DeleteResourceResponse{Success: true}, nil
}

func (s *ServerAPI) ChangeResourceStatus(ctx context.Context, in *resourcev1.ChangeResourceStatusRequest) (*resourcev1.ChangeResourceStatusResponse, error) {
	if in.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource ID is required")
	}

	if in.GetStatus() == resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "resource status is required")
	}

	resource, err := s.service.ChangeResourceStatus(ctx, in.GetResourceId(), protoStatusToService(in.GetStatus()), in.GetReason())
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, models.ErrInvalidStatus) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to change resource status")
	}

	return &resourcev1.ChangeResourceStatusResponse{
		Resource: mapServiceResourceToProto(resource),
	}, nil
}

///////////////////////////////////////////////////////////////////////////////////////////////////

func validateCreateResourceRequest(req *resourcev1.CreateResourceRequest) error {
	if req.GetName() == "" {
		return fmt.Errorf("resource name is required")
	}
	if req.GetType() == resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED {
		return fmt.Errorf("resource type is required")
	}

	switch req.GetType() {
	case resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM:
		if _, ok := req.GetDetails().(*resourcev1.CreateResourceRequest_MeetingRoom); !ok {
			return fmt.Errorf("meeting_room details required for type MEETING_ROOM")
		}
	case resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE:
		if _, ok := req.GetDetails().(*resourcev1.CreateResourceRequest_Workspace); !ok {
			return fmt.Errorf("workspace details required for type WORKSPACE")
		}
	case resourcev1.ResourceType_RESOURCE_TYPE_DEVICE:
		if _, ok := req.GetDetails().(*resourcev1.CreateResourceRequest_Device); !ok {
			return fmt.Errorf("device details required for type DEVICE")
		}
	}

	return nil
}

func protoStatusToService(s resourcev1.ResourceStatus) models.ResourceStatus {
	switch s {
	case resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE:
		return models.ResourceStatusAvailable
	case resourcev1.ResourceStatus_RESOURCE_STATUS_OCCUPIED:
		return models.ResourceStatusOccupied
	case resourcev1.ResourceStatus_RESOURCE_STATUS_MAINTENANCE:
		return models.ResourceStatusMaintenance
	case resourcev1.ResourceStatus_RESOURCE_STATUS_EMERGENCY:
		return models.ResourceStatusEmergency
	default:
		return ""
	}
}

func protoDetailsToService(details any) any {
	switch d := details.(type) {
	case *resourcev1.CreateResourceRequest_MeetingRoom:
		return &models.MeetingRoomDetails{
			Capacity:      d.MeetingRoom.Capacity,
			HasProjector:  d.MeetingRoom.HasProjector,
			HasWhiteboard: d.MeetingRoom.HasWhiteboard,
		}
	case *resourcev1.CreateResourceRequest_Workspace:
		return &models.WorkspaceDetails{
			HasMonitor: d.Workspace.HasMonitor,
		}
	case *resourcev1.CreateResourceRequest_Device:
		return &models.DeviceDetails{
			DeviceType:   d.Device.DeviceType,
			SerialNumber: d.Device.SerialNumber,
			Model:        d.Device.Model,
			Description:  d.Device.Description,
		}
	default:
		return nil
	}
}

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

func buildUpdateResourceRequest(in *resourcev1.UpdateResourceRequest) UpdateResourceRequest {
	var req UpdateResourceRequest
	for _, path := range in.GetFieldMask().GetPaths() {
		switch path {
		case "name":
			req.Name = &in.GetResource().Name
		case "location":
			req.Location = &in.GetResource().Location
		case "details":
			req.Details = protoDetailsToService(in.GetResource().GetDetails())
		}
	}

	return req
}
