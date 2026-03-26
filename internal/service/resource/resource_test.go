package resource

import (
	"context"
	"errors"
	"testing"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
)

type fakeRepository struct {
	createResourceFn         func(ctx context.Context, resource *models.Resource) (*models.Resource, error)
	getResourceFn            func(ctx context.Context, resourceID string) (*models.Resource, error)
	getResourcesListFn       func(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error)
	getAvailableResourcesFn  func(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error)
	updateResourceFn         func(ctx context.Context, resourceID string, in UpdateResourceRequest) (*models.Resource, error)
	deleteResourceFn         func(ctx context.Context, resourceID string) error
	changeResourceStatusFn   func(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error)
}

func (f *fakeRepository) CreateResource(ctx context.Context, resource *models.Resource) (*models.Resource, error) {
	if f.createResourceFn == nil {
		return nil, nil
	}
	return f.createResourceFn(ctx, resource)
}

func (f *fakeRepository) GetResource(ctx context.Context, resourceID string) (*models.Resource, error) {
	if f.getResourceFn == nil {
		return nil, nil
	}
	return f.getResourceFn(ctx, resourceID)
}

func (f *fakeRepository) GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error) {
	if f.getResourcesListFn == nil {
		return nil, nil
	}
	return f.getResourcesListFn(ctx, types)
}

func (f *fakeRepository) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	if f.getAvailableResourcesFn == nil {
		return nil, nil
	}
	return f.getAvailableResourcesFn(ctx, types, location)
}

func (f *fakeRepository) UpdateResource(ctx context.Context, resourceID string, in UpdateResourceRequest) (*models.Resource, error) {
	if f.updateResourceFn == nil {
		return nil, nil
	}
	return f.updateResourceFn(ctx, resourceID, in)
}

func (f *fakeRepository) DeleteResource(ctx context.Context, resourceID string) error {
	if f.deleteResourceFn == nil {
		return nil
	}
	return f.deleteResourceFn(ctx, resourceID)
}

func (f *fakeRepository) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	if f.changeResourceStatusFn == nil {
		return nil, nil
	}
	return f.changeResourceStatusFn(ctx, resourceID, status, reason)
}

func TestServiceCreateResource_DefaultStatus(t *testing.T) {
	t.Parallel()

	var captured *models.Resource
	repo := &fakeRepository{
		createResourceFn: func(_ context.Context, resource *models.Resource) (*models.Resource, error) {
			captured = resource
			return &models.Resource{ID: "res-1", Status: resource.Status}, nil
		},
	}
	svc := NewService(repo, nil)

	_, err := svc.CreateResource(context.Background(), CreateResourceRequest{
		Name:     "Room 101",
		Type:     models.ResourceTypeMeetingRoom,
		Location: "HQ",
		Details:  &models.MeetingRoomDetails{Capacity: 8},
	})
	if err != nil {
		t.Fatalf("CreateResource() error = %v", err)
	}
	if captured == nil {
		t.Fatal("repo CreateResource was not called")
	}
	if captured.Status != models.ResourceStatusAvailable {
		t.Fatalf("expected default status %q, got %q", models.ResourceStatusAvailable, captured.Status)
	}
}

func TestServiceCreateResource_RepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		createResourceFn: func(_ context.Context, _ *models.Resource) (*models.Resource, error) {
			return nil, errors.New("insert failed")
		},
	}
	svc := NewService(repo, nil)

	_, err := svc.CreateResource(context.Background(), CreateResourceRequest{Name: "Room", Type: models.ResourceTypeMeetingRoom})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestServiceUpdateResource_ErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repoErr error
		wantIs  error
	}{
		{name: "not found", repoErr: models.ErrNotFound, wantIs: models.ErrNotFound},
		{name: "invalid type", repoErr: models.ErrInvalidType, wantIs: models.ErrInvalidType},
		{name: "other error", repoErr: errors.New("db failed"), wantIs: errors.New("db failed")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{
				updateResourceFn: func(_ context.Context, _ string, _ UpdateResourceRequest) (*models.Resource, error) {
					return nil, tt.repoErr
				},
			}
			svc := NewService(repo, nil)

			_, err := svc.UpdateResource(context.Background(), "res-1", UpdateResourceRequest{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if tt.name == "other error" {
				if err.Error() == "" {
					t.Fatal("expected wrapped error message")
				}
				return
			}

			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("expected errors.Is(err, %v) to be true; err=%v", tt.wantIs, err)
			}
		})
	}
}

func TestServiceCheckResourceStatus(t *testing.T) {
	t.Parallel()

	t.Run("available", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return &models.Resource{Status: models.ResourceStatusAvailable}, nil
			},
		}
		svc := NewService(repo, nil)

		isAvailable, status, err := svc.CheckResourceStatus(context.Background(), "res-1")
		if err != nil {
			t.Fatalf("CheckResourceStatus() error = %v", err)
		}
		if !isAvailable {
			t.Fatal("expected resource to be available")
		}
		if status != models.ResourceStatusAvailable {
			t.Fatalf("expected status %q, got %q", models.ResourceStatusAvailable, status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return nil, models.ErrNotFound
			},
		}
		svc := NewService(repo, nil)

		_, _, err := svc.CheckResourceStatus(context.Background(), "missing")
		if !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return nil, errors.New("db failed")
			},
		}
		svc := NewService(repo, nil)

		_, _, err := svc.CheckResourceStatus(context.Background(), "res-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestServiceUpdateResourceOccupancy(t *testing.T) {
	t.Parallel()

	t.Run("reject maintenance", func(t *testing.T) {
		t.Parallel()

		called := 0
		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return &models.Resource{Status: models.ResourceStatusMaintenance}, nil
			},
			changeResourceStatusFn: func(_ context.Context, _ string, _ models.ResourceStatus, _ string) (*models.Resource, error) {
				called++
				return nil, nil
			},
		}
		svc := NewService(repo, nil)

		_, err := svc.UpdateResourceOccupancy(context.Background(), "res-1", true)
		if !errors.Is(err, models.ErrInvalidStatus) {
			t.Fatalf("expected ErrInvalidStatus, got %v", err)
		}
		if called != 0 {
			t.Fatalf("expected ChangeResourceStatus not to be called, got %d", called)
		}
	})

	t.Run("no-op when same status", func(t *testing.T) {
		t.Parallel()

		called := 0
		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return &models.Resource{Status: models.ResourceStatusOccupied}, nil
			},
			changeResourceStatusFn: func(_ context.Context, _ string, _ models.ResourceStatus, _ string) (*models.Resource, error) {
				called++
				return &models.Resource{Status: models.ResourceStatusOccupied}, nil
			},
		}
		svc := NewService(repo, nil)

		status, err := svc.UpdateResourceOccupancy(context.Background(), "res-1", true)
		if err != nil {
			t.Fatalf("UpdateResourceOccupancy() error = %v", err)
		}
		if status != models.ResourceStatusOccupied {
			t.Fatalf("expected occupied, got %q", status)
		}
		if called != 0 {
			t.Fatalf("expected ChangeResourceStatus not to be called, got %d", called)
		}
	})

	t.Run("transition available to occupied", func(t *testing.T) {
		t.Parallel()

		called := 0
		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return &models.Resource{Status: models.ResourceStatusAvailable}, nil
			},
			changeResourceStatusFn: func(_ context.Context, _ string, status models.ResourceStatus, _ string) (*models.Resource, error) {
				called++
				if status != models.ResourceStatusOccupied {
					t.Fatalf("expected occupied status, got %q", status)
				}
				return &models.Resource{Status: models.ResourceStatusOccupied}, nil
			},
		}
		svc := NewService(repo, nil)

		status, err := svc.UpdateResourceOccupancy(context.Background(), "res-1", true)
		if err != nil {
			t.Fatalf("UpdateResourceOccupancy() error = %v", err)
		}
		if status != models.ResourceStatusOccupied {
			t.Fatalf("expected occupied, got %q", status)
		}
		if called != 1 {
			t.Fatalf("expected ChangeResourceStatus to be called once, got %d", called)
		}
	})

	t.Run("not found on get resource", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return nil, models.ErrNotFound
			},
		}
		svc := NewService(repo, nil)

		_, err := svc.UpdateResourceOccupancy(context.Background(), "missing", true)
		if !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("change status failed", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return &models.Resource{Status: models.ResourceStatusAvailable}, nil
			},
			changeResourceStatusFn: func(_ context.Context, _ string, _ models.ResourceStatus, _ string) (*models.Resource, error) {
				return nil, errors.New("update failed")
			},
		}
		svc := NewService(repo, nil)

		_, err := svc.UpdateResourceOccupancy(context.Background(), "res-1", true)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("set available when not occupied", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{
			getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
				return &models.Resource{Status: models.ResourceStatusOccupied}, nil
			},
			changeResourceStatusFn: func(_ context.Context, _ string, status models.ResourceStatus, _ string) (*models.Resource, error) {
				if status != models.ResourceStatusAvailable {
					t.Fatalf("expected available status, got %q", status)
				}
				return &models.Resource{Status: models.ResourceStatusAvailable}, nil
			},
		}
		svc := NewService(repo, nil)

		status, err := svc.UpdateResourceOccupancy(context.Background(), "res-1", false)
		if err != nil {
			t.Fatalf("UpdateResourceOccupancy() error = %v", err)
		}
		if status != models.ResourceStatusAvailable {
			t.Fatalf("expected available, got %q", status)
		}
	})
}

func TestServiceGetResource(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return &models.Resource{ID: "res-1"}, nil
		}}
		svc := NewService(repo, nil)

		resource, err := svc.GetResource(context.Background(), "res-1")
		if err != nil {
			t.Fatalf("GetResource() error = %v", err)
		}
		if resource == nil || resource.ID != "res-1" {
			t.Fatalf("unexpected resource: %+v", resource)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{getResourceFn: func(_ context.Context, _ string) (*models.Resource, error) {
			return nil, errors.New("db")
		}}
		svc := NewService(repo, nil)

		_, err := svc.GetResource(context.Background(), "res-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestServiceGetResourcesList(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{getResourcesListFn: func(_ context.Context, _ []models.ResourceType) ([]*models.Resource, error) {
		return []*models.Resource{{ID: "res-1"}}, nil
	}}
	svc := NewService(repo, nil)

	resources, err := svc.GetResourcesList(context.Background(), []models.ResourceType{models.ResourceTypeWorkspace})
	if err != nil {
		t.Fatalf("GetResourcesList() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
}

func TestServiceGetAvailableResources(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{getAvailableResourcesFn: func(_ context.Context, _ []models.ResourceType, _ string) ([]*models.Resource, error) {
			return []*models.Resource{{ID: "res-1"}}, nil
		}}
		svc := NewService(repo, nil)

		resources, err := svc.GetAvailableResources(context.Background(), nil, "")
		if err != nil {
			t.Fatalf("GetAvailableResources() error = %v", err)
		}
		if len(resources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(resources))
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{getAvailableResourcesFn: func(_ context.Context, _ []models.ResourceType, _ string) ([]*models.Resource, error) {
			return nil, errors.New("query failed")
		}}
		svc := NewService(repo, nil)

		_, err := svc.GetAvailableResources(context.Background(), nil, "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestServiceDeleteResource(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{deleteResourceFn: func(_ context.Context, _ string) error { return nil }}
		svc := NewService(repo, nil)

		if err := svc.DeleteResource(context.Background(), "res-1"); err != nil {
			t.Fatalf("DeleteResource() error = %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{deleteResourceFn: func(_ context.Context, _ string) error { return models.ErrNotFound }}
		svc := NewService(repo, nil)

		err := svc.DeleteResource(context.Background(), "missing")
		if !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestServiceChangeResourceStatus(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{changeResourceStatusFn: func(_ context.Context, _ string, status models.ResourceStatus, _ string) (*models.Resource, error) {
			return &models.Resource{Status: status}, nil
		}}
		svc := NewService(repo, nil)

		resource, err := svc.ChangeResourceStatus(context.Background(), "res-1", models.ResourceStatusEmergency, "test")
		if err != nil {
			t.Fatalf("ChangeResourceStatus() error = %v", err)
		}
		if resource.Status != models.ResourceStatusEmergency {
			t.Fatalf("expected emergency status, got %q", resource.Status)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()

		repo := &fakeRepository{changeResourceStatusFn: func(_ context.Context, _ string, _ models.ResourceStatus, _ string) (*models.Resource, error) {
			return nil, models.ErrInvalidStatus
		}}
		svc := NewService(repo, nil)

		_, err := svc.ChangeResourceStatus(context.Background(), "res-1", models.ResourceStatusOccupied, "test")
		if !errors.Is(err, models.ErrInvalidStatus) {
			t.Fatalf("expected ErrInvalidStatus, got %v", err)
		}
	})
}

