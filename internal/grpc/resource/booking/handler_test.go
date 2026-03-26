package booking

import (
	"context"
	"errors"
	"testing"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type bookingServiceStub struct {
	getResourceFn           func(ctx context.Context, resourceID string) (*models.Resource, error)
	checkResourceStatusFn   func(ctx context.Context, resourceID string) (bool, models.ResourceStatus, error)
	getAvailableResourcesFn func(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
	updateOccupancyFn       func(ctx context.Context, resourceID string, isOccupied bool) (models.ResourceStatus, error)
}

func (s *bookingServiceStub) GetResource(ctx context.Context, resourceID string) (*models.Resource, error) {
	if s.getResourceFn == nil {
		return nil, nil
	}
	return s.getResourceFn(ctx, resourceID)
}

func (s *bookingServiceStub) CheckResourceStatus(ctx context.Context, resourceID string) (bool, models.ResourceStatus, error) {
	if s.checkResourceStatusFn == nil {
		return false, "", nil
	}
	return s.checkResourceStatusFn(ctx, resourceID)
}

func (s *bookingServiceStub) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	if s.getAvailableResourcesFn == nil {
		return nil, nil
	}
	return s.getAvailableResourcesFn(ctx, types, location)
}

func (s *bookingServiceStub) UpdateResourceOccupancy(ctx context.Context, resourceID string, isOccupied bool) (models.ResourceStatus, error) {
	if s.updateOccupancyFn == nil {
		return "", nil
	}
	return s.updateOccupancyFn(ctx, resourceID, isOccupied)
}

func TestBookingUpdateResourceOccupancy_InvalidArgument(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &bookingServiceStub{}}
	_, err := h.UpdateResourceOccupancy(context.Background(), &resourcev1.UpdateResourceOccupancyRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestBookingCheckResourceStatus_NotFound(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &bookingServiceStub{
		checkResourceStatusFn: func(_ context.Context, _ string) (bool, models.ResourceStatus, error) {
			return false, "", models.ErrNotFound
		},
	}}

	_, err := h.CheckResourceStatus(context.Background(), &resourcev1.CheckResourceStatusRequest{ResourceId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
}

func TestBookingUpdateResourceOccupancy_FailedPrecondition(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &bookingServiceStub{
		updateOccupancyFn: func(_ context.Context, _ string, _ bool) (models.ResourceStatus, error) {
			return "", models.ErrInvalidStatus
		},
	}}

	_, err := h.UpdateResourceOccupancy(context.Background(), &resourcev1.UpdateResourceOccupancyRequest{ResourceId: "res-1", IsOccupied: true})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v (err=%v)", status.Code(err), err)
	}
}

func TestBookingUpdateResourceOccupancy_Success(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &bookingServiceStub{
		updateOccupancyFn: func(_ context.Context, _ string, _ bool) (models.ResourceStatus, error) {
			return models.ResourceStatusOccupied, nil
		},
	}}

	resp, err := h.UpdateResourceOccupancy(context.Background(), &resourcev1.UpdateResourceOccupancyRequest{ResourceId: "res-1", IsOccupied: true})
	if err != nil {
		t.Fatalf("UpdateResourceOccupancy() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatal("expected success=true")
	}
	if resp.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_OCCUPIED {
		t.Fatalf("expected occupied status, got %v", resp.GetStatus())
	}
}

func TestBookingGetResource(t *testing.T) {
	t.Parallel()

	t.Run("invalid argument", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{}}
		_, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return nil, models.ErrNotFound
		}}}
		_, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{ResourceId: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("internal", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return nil, errors.New("db")
		}}}
		_, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{ResourceId: "res-1"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return &models.Resource{ID: "res-1", Type: models.ResourceTypeWorkspace, Status: models.ResourceStatusAvailable, Details: &models.WorkspaceDetails{}}, nil
		}}}
		resp, err := h.GetResource(context.Background(), &resourcev1.GetResourceRequest{ResourceId: "res-1"})
		if err != nil {
			t.Fatalf("GetResource() error = %v", err)
		}
		if resp.GetResource().GetResourceId() != "res-1" {
			t.Fatalf("unexpected resource_id: %q", resp.GetResource().GetResourceId())
		}
	})
}

func TestBookingCheckResourceStatus(t *testing.T) {
	t.Parallel()

	t.Run("invalid argument", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{}}
		_, err := h.CheckResourceStatus(context.Background(), &resourcev1.CheckResourceStatusRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("internal", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{checkResourceStatusFn: func(_ context.Context, _ string) (bool, models.ResourceStatus, error) {
			return false, "", errors.New("db")
		}}}
		_, err := h.CheckResourceStatus(context.Background(), &resourcev1.CheckResourceStatusRequest{ResourceId: "res-1"})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{checkResourceStatusFn: func(_ context.Context, _ string) (bool, models.ResourceStatus, error) {
			return true, models.ResourceStatusAvailable, nil
		}}}
		resp, err := h.CheckResourceStatus(context.Background(), &resourcev1.CheckResourceStatusRequest{ResourceId: "res-1"})
		if err != nil {
			t.Fatalf("CheckResourceStatus() error = %v", err)
		}
		if !resp.GetIsAvailable() || resp.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}

func TestBookingGetAvailableResources(t *testing.T) {
	t.Parallel()

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{}}
		_, err := h.GetAvailableResources(context.Background(), &resourcev1.GetAvailableResourcesRequest{Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED}})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("internal", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{getAvailableResourcesFn: func(_ context.Context, _ []models.ResourceType, _ string) ([]*models.Resource, error) {
			return nil, errors.New("db")
		}}}
		_, err := h.GetAvailableResources(context.Background(), &resourcev1.GetAvailableResourcesRequest{Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_DEVICE}})
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		h := &ServerAPI{service: &bookingServiceStub{getAvailableResourcesFn: func(_ context.Context, _ []models.ResourceType, _ string) ([]*models.Resource, error) {
			return []*models.Resource{{ID: "res-1", Type: models.ResourceTypeDevice, Status: models.ResourceStatusAvailable, Details: &models.DeviceDetails{DeviceType: "laptop"}}}, nil
		}}}
		resp, err := h.GetAvailableResources(context.Background(), &resourcev1.GetAvailableResourcesRequest{Types: []resourcev1.ResourceType{resourcev1.ResourceType_RESOURCE_TYPE_DEVICE}})
		if err != nil {
			t.Fatalf("GetAvailableResources() error = %v", err)
		}
		if resp.GetTotalCount() != 1 {
			t.Fatalf("expected total_count=1, got %d", resp.GetTotalCount())
		}
	})
}

func TestBookingUpdateResourceOccupancy_InternalError(t *testing.T) {
	t.Parallel()

	h := &ServerAPI{service: &bookingServiceStub{
		updateOccupancyFn: func(_ context.Context, _ string, _ bool) (models.ResourceStatus, error) {
			return "", errors.New("db")
		},
	}}

	_, err := h.UpdateResourceOccupancy(context.Background(), &resourcev1.UpdateResourceOccupancyRequest{ResourceId: "res-1", IsOccupied: true})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v (err=%v)", status.Code(err), err)
	}
}

