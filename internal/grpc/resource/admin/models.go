package admin

import (
	"fmt"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"github.com/acyushka/oregon-resource-service/internal/grpc/resource/utils"
)

type CreateResourceRequest struct {
	Name     string
	Type     models.ResourceType
	Location string
	Details  any
}

type UpdateResourceRequest struct {
	Name     *string
	Type     *models.ResourceType
	Location *string
	Details  any
}

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

func buildUpdateResourceRequest(in *resourcev1.UpdateResourceRequest) UpdateResourceRequest {
	var req UpdateResourceRequest
	for _, path := range in.GetFieldMask().GetPaths() {
		switch path {
		case "name":
			req.Name = &in.GetResource().Name
		case "location":
			req.Location = &in.GetResource().Location
		case "details":
			req.Details = utils.ProtoDetailsToService(in.GetResource().GetDetails())
		}
	}

	return req
}
