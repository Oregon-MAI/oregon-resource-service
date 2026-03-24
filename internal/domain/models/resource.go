package models

import "time"

type Resource struct {
	ID        string
	Name      string
	Type      ResourceType
	Location  string
	Status    ResourceStatus
	Details   any
	CreatedAt time.Time
	UpdatedAt time.Time
}

type MeetingRoomDetails struct {
	Capacity      int32
	HasProjector  bool
	HasWhiteboard bool
}

type WorkspaceDetails struct {
	HasMonitor bool
}

type DeviceDetails struct {
	DeviceType   string
	SerialNumber string
	Model        string
	Description  string
}

type ResourceType string

const (
	ResourceTypeMeetingRoom ResourceType = "meeting_room"
	ResourceTypeWorkspace   ResourceType = "workspace"
	ResourceTypeDevice      ResourceType = "device"
)

type ResourceStatus string

const (
	ResourceStatusAvailable   ResourceStatus = "available"
	ResourceStatusOccupied    ResourceStatus = "occupied"
	ResourceStatusMaintenance ResourceStatus = "maintenance"
	ResourceStatusEmergency   ResourceStatus = "emergency"
)
