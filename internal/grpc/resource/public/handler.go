package public

import (
	"context"
	"errors"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"github.com/acyushka/oregon-resource-service/internal/grpc/resource/utils"
	serviceResource "github.com/acyushka/oregon-resource-service/internal/service/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceServicePublic interface {
	CreateResource(ctx context.Context, in serviceResource.CreateResourceRequest) (*models.Resource, error)
	GetResource(ctx context.Context, resourceID string) (*models.Resource, error)
	GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error)
	UpdateResource(ctx context.Context, resourceID string, in serviceResource.UpdateResourceRequest) (*models.Resource, error)
	DeleteResource(ctx context.Context, resourceID string) error
	ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error)
	GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
}

type ServerAPI struct {
	resourcev1.UnimplementedResourcePublicServiceServer
	service ResourceServicePublic
}

func NewServer(gRPCServer *grpc.Server, service ResourceServicePublic) {
	resourcev1.RegisterResourcePublicServiceServer(gRPCServer, &ServerAPI{service: service})
}

func (s *ServerAPI) CreateResource(ctx context.Context, in *resourcev1.CreateResourceRequest) (*resourcev1.CreateResourceResponse, error) {
	if err := validateCreateResourceRequest(in); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	resourceType, err := utils.ProtoTypeToService(in.GetType())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	req := serviceResource.CreateResourceRequest{
		Name:     in.GetName(),
		Type:     resourceType,
		Location: in.GetLocation(),
		Details:  utils.ProtoDetailsToService(in.GetDetails()),
	}

	resource, err := s.service.CreateResource(ctx, req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &resourcev1.CreateResourceResponse{Resource: utils.MapServiceResourceToProto(resource)}, nil
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

func (s *ServerAPI) GetResourcesList(ctx context.Context, in *resourcev1.GetResourcesListRequest) (*resourcev1.GetResourcesListResponse, error) {
	types, err := utils.ProtoTypesToService(in.GetTypes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	resources, err := s.service.GetResourcesList(ctx, types)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve resources list")
	}

	protoResources := make([]*resourcev1.Resource, len(resources))
	for i, res := range resources {
		protoResources[i] = utils.MapServiceResourceToProto(res)
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

	resource, err := s.service.UpdateResource(ctx, in.GetResourceId(), updateReq)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, models.ErrInvalidType) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to update resource")
	}

	return &resourcev1.UpdateResourceResponse{Resource: utils.MapServiceResourceToProto(resource)}, nil
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

	resource, err := s.service.ChangeResourceStatus(ctx, in.GetResourceId(), utils.ProtoStatusToService(in.GetStatus()), in.GetReason())
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
		Resource: utils.MapServiceResourceToProto(resource),
	}, nil
}
