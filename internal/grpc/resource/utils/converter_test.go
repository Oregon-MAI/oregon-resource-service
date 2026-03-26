package utils

import (
	"testing"
	"time"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
)

func TestProtoTypeToService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      resourcev1.ResourceType
		want    models.ResourceType
		wantErr bool
	}{
		{name: "meeting room", in: resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM, want: models.ResourceTypeMeetingRoom},
		{name: "workspace", in: resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE, want: models.ResourceTypeWorkspace},
		{name: "device", in: resourcev1.ResourceType_RESOURCE_TYPE_DEVICE, want: models.ResourceTypeDevice},
		{name: "unspecified", in: resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ProtoTypeToService(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ProtoTypeToService() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestProtoTypesToService(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		got, err := ProtoTypesToService([]resourcev1.ResourceType{
			resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM,
			resourcev1.ResourceType_RESOURCE_TYPE_DEVICE,
		})
		if err != nil {
			t.Fatalf("ProtoTypesToService() error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 types, got %d", len(got))
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		_, err := ProtoTypesToService([]resourcev1.ResourceType{
			resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE,
			resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestServiceStatusToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   models.ResourceStatus
		want resourcev1.ResourceStatus
	}{
		{name: "available", in: models.ResourceStatusAvailable, want: resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE},
		{name: "occupied", in: models.ResourceStatusOccupied, want: resourcev1.ResourceStatus_RESOURCE_STATUS_OCCUPIED},
		{name: "maintenance", in: models.ResourceStatusMaintenance, want: resourcev1.ResourceStatus_RESOURCE_STATUS_MAINTENANCE},
		{name: "emergency", in: models.ResourceStatusEmergency, want: resourcev1.ResourceStatus_RESOURCE_STATUS_EMERGENCY},
		{name: "default", in: models.ResourceStatus("unknown"), want: resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ServiceStatusToProto(tt.in); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestServiceTypeToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   models.ResourceType
		want resourcev1.ResourceType
	}{
		{name: "meeting_room", in: models.ResourceTypeMeetingRoom, want: resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM},
		{name: "workspace", in: models.ResourceTypeWorkspace, want: resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE},
		{name: "device", in: models.ResourceTypeDevice, want: resourcev1.ResourceType_RESOURCE_TYPE_DEVICE},
		{name: "default", in: models.ResourceType("unknown"), want: resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ServiceTypeToProto(tt.in); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestProtoStatusToService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   resourcev1.ResourceStatus
		want models.ResourceStatus
	}{
		{name: "available", in: resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE, want: models.ResourceStatusAvailable},
		{name: "occupied", in: resourcev1.ResourceStatus_RESOURCE_STATUS_OCCUPIED, want: models.ResourceStatusOccupied},
		{name: "maintenance", in: resourcev1.ResourceStatus_RESOURCE_STATUS_MAINTENANCE, want: models.ResourceStatusMaintenance},
		{name: "emergency", in: resourcev1.ResourceStatus_RESOURCE_STATUS_EMERGENCY, want: models.ResourceStatusEmergency},
		{name: "default", in: resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ProtoStatusToService(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestProtoDetailsToService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     any
		assert func(*testing.T, any)
	}{
		{
			name: "meeting room from create",
			in: &resourcev1.CreateResourceRequest_MeetingRoom{
				MeetingRoom: &resourcev1.MeetingRoomDetails{Capacity: 12, HasProjector: true, HasWhiteboard: true},
			},
			assert: func(t *testing.T, got any) { assertMeetingRoomDetails(t, got, 12, true, true) },
		},
		{
			name:   "workspace from create",
			in:     &resourcev1.CreateResourceRequest_Workspace{Workspace: &resourcev1.WorkspaceDetails{HasMonitor: true}},
			assert: func(t *testing.T, got any) { assertWorkspaceDetails(t, got, true) },
		},
		{
			name:   "device from create",
			in:     &resourcev1.CreateResourceRequest_Device{Device: &resourcev1.DeviceDetails{DeviceType: "laptop", Model: "m1"}},
			assert: func(t *testing.T, got any) { assertDeviceDetails(t, got, "laptop", "m1") },
		},
		{
			name:   "workspace from resource",
			in:     &resourcev1.Resource_Workspace{Workspace: &resourcev1.WorkspaceDetails{HasMonitor: true}},
			assert: func(t *testing.T, got any) { assertWorkspaceDetails(t, got, true) },
		},
		{
			name:   "meeting room from resource",
			in:     &resourcev1.Resource_MeetingRoom{MeetingRoom: &resourcev1.MeetingRoomDetails{Capacity: 4}},
			assert: func(t *testing.T, got any) { assertMeetingRoomDetails(t, got, 4, false, false) },
		},
		{
			name:   "device from resource",
			in:     &resourcev1.Resource_Device{Device: &resourcev1.DeviceDetails{DeviceType: "monitor", Model: "m2"}},
			assert: func(t *testing.T, got any) { assertDeviceDetails(t, got, "monitor", "m2") },
		},
		{
			name: "unknown details",
			in:   nil,
			assert: func(t *testing.T, got any) {
				t.Helper()
				if got != nil {
					t.Fatalf("expected nil, got %T", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.assert(t, ProtoDetailsToService(tt.in))
		})
	}
}

func assertMeetingRoomDetails(t *testing.T, got any, capacity int32, hasProjector bool, hasWhiteboard bool) {
	t.Helper()

	d, ok := got.(*models.MeetingRoomDetails)
	if !ok {
		t.Fatalf("expected *models.MeetingRoomDetails, got %T", got)
	}
	if d.Capacity != capacity || d.HasProjector != hasProjector || d.HasWhiteboard != hasWhiteboard {
		t.Fatalf("unexpected meeting room details: %+v", d)
	}
}

func assertWorkspaceDetails(t *testing.T, got any, hasMonitor bool) {
	t.Helper()

	d, ok := got.(*models.WorkspaceDetails)
	if !ok {
		t.Fatalf("expected *models.WorkspaceDetails, got %T", got)
	}
	if d.HasMonitor != hasMonitor {
		t.Fatalf("unexpected workspace details: %+v", d)
	}
}

func assertDeviceDetails(t *testing.T, got any, deviceType string, model string) {
	t.Helper()

	d, ok := got.(*models.DeviceDetails)
	if !ok {
		t.Fatalf("expected *models.DeviceDetails, got %T", got)
	}
	if d.DeviceType != deviceType || d.Model != model {
		t.Fatalf("unexpected device details: %+v", d)
	}
}

func TestMapServiceResourceToProto(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	res := &models.Resource{
		ID:        "res-1",
		Name:      "Workspace A",
		Type:      models.ResourceTypeWorkspace,
		Location:  "HQ",
		Status:    models.ResourceStatusAvailable,
		Details:   &models.WorkspaceDetails{HasMonitor: true},
		CreatedAt: now,
		UpdatedAt: now,
	}

	got := MapServiceResourceToProto(res)
	if got.GetResourceId() != "res-1" {
		t.Fatalf("expected resource_id res-1, got %q", got.GetResourceId())
	}
	if got.GetType() != resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE {
		t.Fatalf("unexpected type: %v", got.GetType())
	}
	if got.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
		t.Fatalf("unexpected status: %v", got.GetStatus())
	}
	if got.GetWorkspace() == nil || !got.GetWorkspace().GetHasMonitor() {
		t.Fatalf("unexpected workspace details: %+v", got.GetWorkspace())
	}
}

func TestMapServiceResourceToProto_DetailsByType(t *testing.T) {
	t.Parallel()

	meeting := MapServiceResourceToProto(&models.Resource{
		ID:      "m1",
		Type:    models.ResourceTypeMeetingRoom,
		Status:  models.ResourceStatusAvailable,
		Details: &models.MeetingRoomDetails{Capacity: 6},
	})
	if meeting.GetMeetingRoom() == nil || meeting.GetMeetingRoom().GetCapacity() != 6 {
		t.Fatalf("unexpected meeting room mapping: %+v", meeting.GetMeetingRoom())
	}

	device := MapServiceResourceToProto(&models.Resource{
		ID:      "d1",
		Type:    models.ResourceTypeDevice,
		Status:  models.ResourceStatusAvailable,
		Details: &models.DeviceDetails{DeviceType: "laptop", Model: "m2"},
	})
	if device.GetDevice() == nil || device.GetDevice().GetDeviceType() != "laptop" {
		t.Fatalf("unexpected device mapping: %+v", device.GetDevice())
	}
}
