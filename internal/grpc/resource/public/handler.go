package public

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

type ResourceServicePublic interface {
	GetResource(ctx context.Context, resourceID string) (*models.Resource, error)
	GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
}

type ServerAPI struct {
	resourcev1.UnimplementedResourcePublicServiceServer
	service ResourceServicePublic
}

func NewServer(gRPCServer *grpc.Server, service ResourceServicePublic) {
	resourcev1.RegisterResourcePublicServiceServer(gRPCServer, &ServerAPI{service: service})
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
		protoResources[i] = utils.MapServiceResourceToProto(res)
	}

	return &resourcev1.GetAvailableResourcesResponse{
		Resources:  protoResources,
		TotalCount: int32(len(protoResources)),
	}, nil
}
