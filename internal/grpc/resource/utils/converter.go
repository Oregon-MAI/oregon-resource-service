package utils

import (
	"errors"

	resourcev1 "github.com/acyushka/oregon-infra/contracts/gen/go/resource"
	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func MapServiceResourceToProto(res *models.Resource) *resourcev1.Resource {
	proto := &resourcev1.Resource{
		ResourceId: res.ID,
		Name:       res.Name,
		Type:       ServiceTypeToProto(res.Type),
		Location:   res.Location,
		Status:     ServiceStatusToProto(res.Status),
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

func ServiceStatusToProto(s models.ResourceStatus) resourcev1.ResourceStatus {
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

func ServiceTypeToProto(t models.ResourceType) resourcev1.ResourceType {
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

func ProtoTypeToService(t resourcev1.ResourceType) (models.ResourceType, error) {
	switch t {
	case resourcev1.ResourceType_RESOURCE_TYPE_MEETING_ROOM:
		return models.ResourceTypeMeetingRoom, nil
	case resourcev1.ResourceType_RESOURCE_TYPE_WORKSPACE:
		return models.ResourceTypeWorkspace, nil
	case resourcev1.ResourceType_RESOURCE_TYPE_DEVICE:
		return models.ResourceTypeDevice, nil
	default:
		return "", errors.New("invalid resource type")
	}
}

func ProtoTypesToService(types []resourcev1.ResourceType) ([]models.ResourceType, error) {
	serviceTypes := make([]models.ResourceType, 0, len(types))
	for _, t := range types {
		mappedType, err := ProtoTypeToService(t)
		if err != nil {
			return nil, err
		}
		serviceTypes = append(serviceTypes, mappedType)
	}

	return serviceTypes, nil
}

func ProtoStatusToService(s resourcev1.ResourceStatus) models.ResourceStatus {
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

func ProtoDetailsToService(details any) any {
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
	case *resourcev1.Resource_MeetingRoom:
		return &models.MeetingRoomDetails{
			Capacity:      d.MeetingRoom.Capacity,
			HasProjector:  d.MeetingRoom.HasProjector,
			HasWhiteboard: d.MeetingRoom.HasWhiteboard,
		}
	case *resourcev1.Resource_Workspace:
		return &models.WorkspaceDetails{
			HasMonitor: d.Workspace.HasMonitor,
		}
	case *resourcev1.Resource_Device:
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
