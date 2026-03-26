package public

import (
	"context"
	"errors"
	"testing"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	serviceResource "github.com/acyushka/oregon-resource-service/internal/service/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type publicServiceStub struct {
	createResourceFn        func(ctx context.Context, in serviceResource.CreateResourceRequest) (*models.Resource, error)
	getResourceFn           func(ctx context.Context, resourceID string) (*models.Resource, error)
	getResourcesListFn      func(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error)
	updateResourceFn        func(ctx context.Context, resourceID string, in serviceResource.UpdateResourceRequest) (*models.Resource, error)
	deleteResourceFn        func(ctx context.Context, resourceID string) error
	changeResourceStatusFn  func(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error)
	getAvailableResourcesFn func(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
}

func (s *publicServiceStub) CreateResource(ctx context.Context, in serviceResource.CreateResourceRequest) (*models.Resource, error) {
	if s.createResourceFn == nil {
		return nil, nil
	}
	return s.createResourceFn(ctx, in)
}

func (s *publicServiceStub) GetResource(ctx context.Context, resourceID string) (*models.Resource, error) {
	if s.getResourceFn == nil {
		return nil, nil
	}
	return s.getResourceFn(ctx, resourceID)
}

func (s *publicServiceStub) GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error) {
	if s.getResourcesListFn == nil {
		return nil, nil
	}
	return s.getResourcesListFn(ctx, types)
}

func (s *publicServiceStub) UpdateResource(ctx context.Context, resourceID string, in serviceResource.UpdateResourceRequest) (*models.Resource, error) {
	if s.updateResourceFn == nil {
		return nil, nil
	}
	return s.updateResourceFn(ctx, resourceID, in)
}

func (s *publicServiceStub) DeleteResource(ctx context.Context, resourceID string) error {
	if s.deleteResourceFn == nil {
		return nil
	}
	return s.deleteResourceFn(ctx, resourceID)
}

func (s *publicServiceStub) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	if s.changeResourceStatusFn == nil {
		return nil, nil
	}
	return s.changeResourceStatusFn(ctx, resourceID, status, reason)
}

func (s *publicServiceStub) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	if s.getAvailableResourcesFn == nil {
		return nil, nil
	}
	return s.getAvailableResourcesFn(ctx, types, location)
}

func TestPublicGetResource_InvalidArgument(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &publicServiceStub{}}

	_, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestPublicGetResource_NotFound(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &publicServiceStub{
		getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return nil, models.ErrNotFound
		},
	}}

	_, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{ResourceId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
}

func TestPublicChangeResourceStatus_ValidationAndMapping(t *testing.T) {
	t.Parallel()

	t.Run("invalid status input", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.ChangeResourceStatus(context.Background(), &resourcev1.ChangeResourceStatusRequest{ResourceId: "res-1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("service invalid status", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			changeResourceStatusFn: func(_ context.Context, _ string, _ models.ResourceStatus, _ string) (*models.Resource, error) {
				return nil, models.ErrInvalidStatus
			},
		}}

		_, err := h.ChangeResourceStatus(context.Background(), &resourcev1.ChangeResourceStatusRequest{
			ResourceId: "res-1",
			Status:     resourcev1.ResourceStatus_RESOURCE_STATUS_OCCUPIED,
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected FailedPrecondition, got %v (err=%v)", status.Code(err), err)
		}
	})
}

func TestPublicGetResource_InternalError(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &publicServiceStub{
		getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return nil, errors.New("db down")
		},
	}}

	_, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{ResourceId: "res-1"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
	}
}

func TestPublicCreateResource(t *testing.T) {
	t.Parallel()

	t.Run("invalid request", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.CreateResource(context.Background(), &resourcev1.CreateResourceRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			createResourceFn: func(_ context.Context, _ serviceResource.CreateResourceRequest) (*models.Resource, error) {
				return nil, errors.New("insert failed")
			},
		}}

		_, err := h.CreateResource(context.Background(), &resourcev1.CreateResourceRequest{
			Name: "Room",
			Type: resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM,
			Details: &resourcev1.CreateResourceRequest_MeetingRoom{
				MeetingRoom: &resourcev1.MeetingRoomDetails{Capacity: 8},
			},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			createResourceFn: func(_ context.Context, in serviceResource.CreateResourceRequest) (*models.Resource, error) {
				if in.Type != models.ResourceTypeMeetingRoom {
					t.Fatalf("unexpected mapped type: %q", in.Type)
				}
				return &models.Resource{
					ID:       "res-1",
					Name:     in.Name,
					Type:     in.Type,
					Location: in.Location,
					Status:   models.ResourceStatusAvailable,
					Details:  in.Details,
				}, nil
			},
		}}

		resp, err := h.CreateResource(context.Background(), &resourcev1.CreateResourceRequest{
			Name:     "Room",
			Type:     resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM,
			Location: "HQ",
			Details: &resourcev1.CreateResourceRequest_MeetingRoom{
				MeetingRoom: &resourcev1.MeetingRoomDetails{Capacity: 8},
			},
		})
		if err != nil {
			t.Fatalf("CreateResource() error = %v", err)
		}
		if resp.GetResource().GetResourceId() != "res-1" {
			t.Fatalf("unexpected resource id: %q", resp.GetResource().GetResourceId())
		}
	})
}

func TestPublicGetAvailableResources(t *testing.T) {
	t.Parallel()

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.GetAvailableResources(context.Background(), &resourcev1.GetAvailableResourcesRequest{
			Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			getAvailableResourcesFn: func(_ context.Context, _ []models.ResourceType, _ string) ([]*models.Resource, error) {
				return nil, errors.New("query failed")
			},
		}}

		_, err := h.GetAvailableResources(context.Background(), &resourcev1.GetAvailableResourcesRequest{
			Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE},
		})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			getAvailableResourcesFn: func(_ context.Context, _ []models.ResourceType, _ string) ([]*models.Resource, error) {
				return []*models.Resource{{ID: "res-1", Type: models.ResourceTypeWorkspace, Status: models.ResourceStatusAvailable}}, nil
			},
		}}

		resp, err := h.GetAvailableResources(context.Background(), &resourcev1.GetAvailableResourcesRequest{
			Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE},
		})
		if err != nil {
			t.Fatalf("GetAvailableResources() error = %v", err)
		}
		if resp.GetTotalCount() != 1 {
			t.Fatalf("expected total_count=1, got %d", resp.GetTotalCount())
		}
	})
}

func TestPublicGetResourcesList(t *testing.T) {
	t.Parallel()

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.GetResourcesList(context.Background(), &resourcev1.GetResourcesListRequest{
			Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			getResourcesListFn: func(_ context.Context, _ []models.ResourceType) ([]*models.Resource, error) {
				return nil, errors.New("query failed")
			},
		}}
		_, err := h.GetResourcesList(context.Background(), &resourcev1.GetResourcesListRequest{
			Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_DEVICE},
		})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			getResourcesListFn: func(_ context.Context, _ []models.ResourceType) ([]*models.Resource, error) {
				return []*models.Resource{{ID: "res-1", Type: models.ResourceTypeDevice, Status: models.ResourceStatusAvailable}}, nil
			},
		}}
		resp, err := h.GetResourcesList(context.Background(), &resourcev1.GetResourcesListRequest{
			Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_DEVICE},
		})
		if err != nil {
			t.Fatalf("GetResourcesList() error = %v", err)
		}
		if len(resp.GetResources()) != 1 {
			t.Fatalf("expected one resource, got %d", len(resp.GetResources()))
		}
	})
}

func TestPublicUpdateResource(t *testing.T) {
	t.Parallel()

	t.Run("invalid resource id", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.UpdateResource(context.Background(), &resourcev1.UpdateResourceRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("empty field mask", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.UpdateResource(context.Background(), &resourcev1.UpdateResourceRequest{ResourceId: "res-1"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("service not found", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			updateResourceFn: func(_ context.Context, _ string, _ serviceResource.UpdateResourceRequest) (*models.Resource, error) {
				return nil, models.ErrNotFound
			},
		}}
		_, err := h.UpdateResource(context.Background(), &resourcev1.UpdateResourceRequest{
			ResourceId: "res-1",
			Resource:   &resourcev1.Resource{Name: "N"},
			FieldMask:  &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("service invalid type", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			updateResourceFn: func(_ context.Context, _ string, _ serviceResource.UpdateResourceRequest) (*models.Resource, error) {
				return nil, models.ErrInvalidType
			},
		}}
		_, err := h.UpdateResource(context.Background(), &resourcev1.UpdateResourceRequest{
			ResourceId: "res-1",
			Resource:   &resourcev1.Resource{Name: "N"},
			FieldMask:  &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{
			updateResourceFn: func(_ context.Context, _ string, _ serviceResource.UpdateResourceRequest) (*models.Resource, error) {
				return &models.Resource{ID: "res-1", Name: "N", Type: models.ResourceTypeWorkspace, Status: models.ResourceStatusAvailable, Details: &models.WorkspaceDetails{}}, nil
			},
		}}
		resp, err := h.UpdateResource(context.Background(), &resourcev1.UpdateResourceRequest{
			ResourceId: "res-1",
			Resource:   &resourcev1.Resource{Name: "N"},
			FieldMask:  &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		if err != nil {
			t.Fatalf("UpdateResource() error = %v", err)
		}
		if resp.GetResource().GetResourceId() != "res-1" {
			t.Fatalf("unexpected resource_id: %q", resp.GetResource().GetResourceId())
		}
	})
}

func TestPublicDeleteResource(t *testing.T) {
	t.Parallel()

	t.Run("invalid resource id", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{}}
		_, err := h.DeleteResource(context.Background(), &resourcev1.DeleteResourceRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{deleteResourceFn: func(_ context.Context, _ string) error { return models.ErrNotFound }}}
		_, err := h.DeleteResource(context.Background(), &resourcev1.DeleteResourceRequest{ResourceId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("internal", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{deleteResourceFn: func(_ context.Context, _ string) error { return errors.New("db") }}}
		_, err := h.DeleteResource(context.Background(), &resourcev1.DeleteResourceRequest{ResourceId: "res-1"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &publicServiceStub{deleteResourceFn: func(_ context.Context, _ string) error { return nil }}}
		resp, err := h.DeleteResource(context.Background(), &resourcev1.DeleteResourceRequest{ResourceId: "res-1"})
		if err != nil {
			t.Fatalf("DeleteResource() error = %v", err)
		}
		if !resp.GetSuccess() {
			t.Fatal("expected success=true")
		}
	})
}

