package public

import (
	"testing"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestValidateCreateResourceRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *resourcev1.CreateResourceRequest
		wantErr bool
	}{
		{
			name:    "missing name",
			req:     &resourcev1.CreateResourceRequest{Type: resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE},
			wantErr: true,
		},
		{
			name: "type meeting room but wrong details",
			req: &resourcev1.CreateResourceRequest{
				Name: "Room",
				Type: resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM,
				Details: &resourcev1.CreateResourceRequest_Workspace{
					Workspace: &resourcev1.WorkspaceDetails{HasMonitor: true},
				},
			},
			wantErr: true,
		},
		{
			name: "valid meeting room",
			req: &resourcev1.CreateResourceRequest{
				Name: "Room",
				Type: resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM,
				Details: &resourcev1.CreateResourceRequest_MeetingRoom{
					MeetingRoom: &resourcev1.MeetingRoomDetails{Capacity: 10},
				},
			},
			wantErr: false,
		},
		{
			name: "valid workspace",
			req: &resourcev1.CreateResourceRequest{
				Name: "Desk",
				Type: resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE,
				Details: &resourcev1.CreateResourceRequest_Workspace{
					Workspace: &resourcev1.WorkspaceDetails{HasMonitor: true},
				},
			},
			wantErr: false,
		},
		{
			name: "valid device",
			req: &resourcev1.CreateResourceRequest{
				Name: "Laptop",
				Type: resourcev1.ResourceType_RESOURCE_TYPE_DEVICE,
				Details: &resourcev1.CreateResourceRequest_Device{
					Device: &resourcev1.DeviceDetails{DeviceType: "notebook"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing type",
			req: &resourcev1.CreateResourceRequest{
				Name: "Bad",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateCreateResourceRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateCreateResourceRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildUpdateResourceRequest(t *testing.T) {
	t.Parallel()

	name := "New name"
	location := "New location"
	in := &resourcev1.UpdateResourceRequest{
		ResourceId: "res-1",
		Resource: &resourcev1.Resource{
			Name:     name,
			Location: location,
			Details: &resourcev1.Resource_Workspace{
				Workspace: &resourcev1.WorkspaceDetails{HasMonitor: true},
			},
		},
		FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"name", "location", "details"}},
	}

	out := buildUpdateResourceRequest(in)

	if out.Name == nil || *out.Name != name {
		t.Fatalf("expected Name to be set to %q", name)
	}
	if out.Location == nil || *out.Location != location {
		t.Fatalf("expected Location to be set to %q", location)
	}
	if out.Details == nil {
		t.Fatal("expected Details to be set")
	}
}

func TestBuildUpdateResourceRequest_UnknownMaskPathIgnored(t *testing.T) {
	t.Parallel()

	in := &resourcev1.UpdateResourceRequest{
		Resource:  &resourcev1.Resource{Name: "n"},
		FieldMask: &fieldmaskpb.FieldMask{Paths: []string{"unknown_field"}},
	}

	out := buildUpdateResourceRequest(in)
	if out.Name != nil || out.Location != nil || out.Details != nil {
		t.Fatalf("expected zero-value update request, got %+v", out)
	}
}

