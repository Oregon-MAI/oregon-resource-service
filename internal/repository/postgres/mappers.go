package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	"github.com/lib/pq"
)

type resourceRow struct {
	id, name, typeRaw, statusRaw string
	location                     sql.NullString
	createdAt, updatedAt         sql.NullTime

	mrCapacity      sql.NullInt16
	mrHasProjector  sql.NullBool
	mrHasWhiteboard sql.NullBool

	wsHasMonitor sql.NullBool

	deviceType   sql.NullString
	serialNumber sql.NullString
	model        sql.NullString
	description  sql.NullString
}

func scanResourceRow(row *sql.Row) (resourceRow, error) {
	var r resourceRow
	err := row.Scan(
		&r.id,
		&r.name,
		&r.typeRaw,
		&r.location,
		&r.statusRaw,
		&r.createdAt,
		&r.updatedAt,
		&r.mrCapacity,
		&r.mrHasProjector,
		&r.mrHasWhiteboard,
		&r.wsHasMonitor,
		&r.deviceType,
		&r.serialNumber,
		&r.model,
		&r.description,
	)

	return r, err
}

func parseResourceType(v string) (models.ResourceType, error) {
	t := models.ResourceType(v)
	switch t {
	case models.ResourceTypeMeetingRoom, models.ResourceTypeWorkspace, models.ResourceTypeDevice:
		return t, nil
	default:
		return "", models.ErrInvalidType
	}
}

func parseResourceStatus(v string) (models.ResourceStatus, error) {
	s := models.ResourceStatus(v)
	switch s {
	case models.ResourceStatusAvailable, models.ResourceStatusOccupied, models.ResourceStatusMaintenance, models.ResourceStatusEmergency:
		return s, nil
	default:
		return "", models.ErrInvalidStatus
	}
}

func parseResourceDetails(resourceType models.ResourceType, row resourceRow) (any, error) {
	switch resourceType {
	case models.ResourceTypeMeetingRoom:
		if !row.mrCapacity.Valid {
			return nil, fmt.Errorf("%w: meeting_room details row not found", models.ErrInvalidType)
		}
		return &models.MeetingRoomDetails{
			Capacity:      int32(row.mrCapacity.Int16),
			HasProjector:  row.mrHasProjector.Valid && row.mrHasProjector.Bool,
			HasWhiteboard: row.mrHasWhiteboard.Valid && row.mrHasWhiteboard.Bool,
		}, nil
	case models.ResourceTypeWorkspace:
		if !row.wsHasMonitor.Valid {
			return nil, fmt.Errorf("%w: workspace details row not found", models.ErrInvalidType)
		}
		return &models.WorkspaceDetails{HasMonitor: row.wsHasMonitor.Bool}, nil
	case models.ResourceTypeDevice:
		if !row.deviceType.Valid {
			return nil, fmt.Errorf("%w: device details row not found", models.ErrInvalidType)
		}
		d := &models.DeviceDetails{DeviceType: row.deviceType.String}
		if row.serialNumber.Valid {
			d.SerialNumber = row.serialNumber.String
		}
		if row.model.Valid {
			d.Model = row.model.String
		}
		if row.description.Valid {
			d.Description = row.description.String
		}
		return d, nil
	default:
		return nil, models.ErrInvalidType
	}
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func mapSQLError(err error) error {
	if err == nil {
		return nil
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch string(pqErr.Code) {
		case "22P02":
			return fmt.Errorf("invalid uuid")
		case "23505":
			return fmt.Errorf("unique constraint violation")
		case "23514":
			return fmt.Errorf("check constraint violation")
		case "23503":
			return fmt.Errorf("foreign key violation")
		}
	}

	return err
}

