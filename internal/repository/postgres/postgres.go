package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	service "github.com/acyushka/oregon-resource-service/internal/service/resource"
	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

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

func New(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres.New: %w", err)
	}

	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres.New: %w", err)
	}

	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) CreateResource(ctx context.Context, resource *models.Resource) (*models.Resource, error) {
	const op = "postgres.CreateResource"

	if resource == nil {
		return nil, fmt.Errorf("%s: resource is nil", op)
	}

	transaction, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer func() {
		if err != nil {
			_ = transaction.Rollback()
		}
	}()

	createdResource := &models.Resource{}
	err = transaction.QueryRowContext(ctx, `
		INSERT INTO resources (name, type, location, status)
		VALUES ($1, $2, $3, $4)
		RETURNING uuid::text, name, type::text, location, status::text, created_at, updated_at
	`, resource.Name, resource.Type, resource.Location, resource.Status).Scan(
		&createdResource.ID,
		&createdResource.Name,
		&createdResource.Type,
		&createdResource.Location,
		&createdResource.Status,
		&createdResource.CreatedAt,
		&createdResource.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: insert resources: %w", op, mapSQLError(err))
	}

	if err = insertDetailsByType(ctx, transaction, createdResource.ID, createdResource.Type, resource.Details); err != nil {
		return nil, fmt.Errorf("%s: insert details: %w", op, err)
	}

	if err = transaction.Commit(); err != nil {
		return nil, fmt.Errorf("%s: commit transaction: %w", op, err)
	}

	createdResource.Details = resource.Details
	return createdResource, nil
}

func (r *Repository) GetResource(ctx context.Context, resourceID string) (*models.Resource, error) {
	const op = "postgres.GetResource"

	const query = `
		SELECT
			r.uuid::text,
			r.name,
			r.type::text,
			r.location,
			r.status::text,
			r.created_at,
			r.updated_at,
			mr.capacity,
			mr.has_projector,
			mr.has_whiteboard,
			ws.has_monitor,
			d.device_type,
			d.serial_number,
			d.model,
			d.description
		FROM resources r
		LEFT JOIN meeting_rooms mr ON mr.resource_uuid = r.uuid
		LEFT JOIN workspaces ws ON ws.resource_uuid = r.uuid
		LEFT JOIN devices d ON d.resource_uuid = r.uuid
		WHERE r.uuid = $1::uuid
	`

	scannedResource, err := scanResourceRow(r.db.QueryRowContext(ctx, query, resourceID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	resourceType, err := parseResourceType(scannedResource.typeRaw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	resourceStatus, err := parseResourceStatus(scannedResource.statusRaw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	resource := &models.Resource{
		ID:     scannedResource.id,
		Name:   scannedResource.name,
		Type:   resourceType,
		Status: resourceStatus,
	}
	if scannedResource.location.Valid {
		resource.Location = scannedResource.location.String
	}
	if scannedResource.createdAt.Valid {
		resource.CreatedAt = scannedResource.createdAt.Time
	}
	if scannedResource.updatedAt.Valid {
		resource.UpdatedAt = scannedResource.updatedAt.Time
	}

	resource.Details, err = parseResourceDetails(resourceType, scannedResource)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return resource, nil
}

func (r *Repository) GetResourcesList(ctx context.Context, types []models.ResourceType) ([]*models.Resource, error) {
	return nil, fmt.Errorf("postgres.GetResourcesList: not implemented")
}

func (r *Repository) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	return nil, fmt.Errorf("postgres.GetAvailableResources: not implemented")
}

func (r *Repository) UpdateResource(ctx context.Context, resourceID string, in service.UpdateResourceRequest, fields []string) (*models.Resource, error) {
	return nil, fmt.Errorf("postgres.UpdateResource: not implemented")
}

func (r *Repository) DeleteResource(ctx context.Context, resourceID string) error {
	return fmt.Errorf("postgres.DeleteResource: not implemented")
}

func (r *Repository) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	return nil, fmt.Errorf("postgres.ChangeResourceStatus: not implemented")
}

/////////////////////////////////////////////////////////////////////

func insertDetailsByType(ctx context.Context, tx *sql.Tx, resourceID string, resourceType models.ResourceType, details any) error {
	switch resourceType {
	case models.ResourceTypeMeetingRoom:
		d, ok := details.(*models.MeetingRoomDetails)
		if !ok || d == nil {
			return fmt.Errorf("%w: meeting_room details required", models.ErrInvalidType)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO meeting_rooms (resource_uuid, capacity, has_projector, has_whiteboard)
			VALUES ($1::uuid, $2, $3, $4)
		`, resourceID, d.Capacity, d.HasProjector, d.HasWhiteboard)
		if err != nil {
			return mapSQLError(err)
		}
		return nil
	case models.ResourceTypeWorkspace:
		d, ok := details.(*models.WorkspaceDetails)
		if !ok || d == nil {
			return fmt.Errorf("%w: workspace details required", models.ErrInvalidType)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO workspaces (resource_uuid, has_monitor)
			VALUES ($1::uuid, $2)
		`, resourceID, d.HasMonitor)
		if err != nil {
			return mapSQLError(err)
		}
		return nil
	case models.ResourceTypeDevice:
		d, ok := details.(*models.DeviceDetails)
		if !ok || d == nil {
			return fmt.Errorf("%w: device details required", models.ErrInvalidType)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO devices (resource_uuid, device_type, serial_number, model, description)
			VALUES ($1::uuid, $2, $3, $4, $5)
		`, resourceID, d.DeviceType, nullableString(d.SerialNumber), nullableString(d.Model), nullableString(d.Description))
		if err != nil {
			return mapSQLError(err)
		}
		return nil
	default:
		return models.ErrInvalidType
	}
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
