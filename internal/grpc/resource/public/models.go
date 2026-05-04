package public

import (
	"errors"
	"strings"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/grpc/resource/utils"
	serviceResource "github.com/acyushka/oregon-resource-service/internal/service/resource"
)

func validateCreateResourceRequest(req *resourcev1.CreateResourceRequest) error {
	if req.GetName() == "" {
		return errors.New("resource name is required")
	}
	if req.GetType() == resourcev1.ResourceType_RESOURCE_TYPE_UNSPECIFIED {
		return errors.New("resource type is required")
	}

	switch req.GetType() {
	case resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM:
		if _, ok := req.GetDetails().(*resourcev1.CreateResourceRequest_MeetingRoom); !ok {
			return errors.New("meeting_room details required for type MEETING_ROOM")
		}
	case resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE:
		if _, ok := req.GetDetails().(*resourcev1.CreateResourceRequest_Workspace); !ok {
			return errors.New("workspace details required for type WORKSPACE")
		}
	case resourcev1.ResourceType_RESOURCE_TYPE_DEVICE:
		if _, ok := req.GetDetails().(*resourcev1.CreateResourceRequest_Device); !ok {
			return errors.New("device details required for type DEVICE")
		}
	}

	return nil
}

func buildUpdateResourceRequest(in *resourcev1.UpdateResourceRequest) serviceResource.UpdateResourceRequest {
	var req serviceResource.UpdateResourceRequest
	for _, path := range in.GetFieldMask().GetPaths() {
		switch {
		case path == "name":
			req.Name = &in.GetResource().Name
		case path == "location":
			req.Location = &in.GetResource().Location
		case path == "details" || strings.HasPrefix(path, "details."):
			req.Details = utils.ProtoDetailsToService(in.GetResource().GetDetails())
		}
	}

	return req
}
