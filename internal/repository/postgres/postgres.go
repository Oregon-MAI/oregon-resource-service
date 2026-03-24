package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	service "github.com/acyushka/oregon-resource-service/internal/service/resource"
)

type Repository struct {
	db *sql.DB
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
	const op = "postgres.GetResourcesList"

	var typeFilter any
	if len(types) > 0 {
		typeValues := make([]string, 0, len(types))
		for _, t := range types {
			typeValues = append(typeValues, string(t))
		}
		typeFilter = pq.Array(typeValues)
	}

	return r.queryResources(ctx, op, `
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
		WHERE ($1::text[] IS NULL OR r.type::text = ANY($1::text[]))
		ORDER BY r.created_at DESC
	`, typeFilter)
}

func (r *Repository) GetAvailableResources(ctx context.Context, types []models.ResourceType, location string) ([]*models.Resource, error) {
	const op = "postgres.GetAvailableResources"

	var typeFilter any
	if len(types) > 0 {
		typeValues := make([]string, 0, len(types))
		for _, t := range types {
			typeValues = append(typeValues, string(t))
		}
		typeFilter = pq.Array(typeValues)
	}

	return r.queryResources(ctx, op, `
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
		WHERE r.status = $2::resource_status
			AND ($1::text[] IS NULL OR r.type::text = ANY($1::text[]))
			AND ($3::text = '' OR r.location = $3)
		ORDER BY r.created_at DESC
	`, typeFilter, models.ResourceStatusAvailable, location)
}

func (r *Repository) UpdateResource(ctx context.Context, resourceID string, in service.UpdateResourceRequest) (*models.Resource, error) {
	const op = "postgres.UpdateResource"

	transaction, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer func() {
		if err != nil {
			_ = transaction.Rollback()
		}
	}()

	hasName := in.Name != nil
	hasLocation := in.Location != nil
	hasDetails := in.Details != nil

	if hasName || hasLocation || hasDetails {
		var name any
		var location any
		if hasName {
			name = *in.Name
		}
		if hasLocation {
			location = *in.Location
		}

		result, err := transaction.ExecContext(ctx, `
			UPDATE resources
			SET
				name = CASE WHEN $2 THEN $3 ELSE name END,
				location = CASE WHEN $4 THEN $5 ELSE location END,
				updated_at = now()
			WHERE uuid = $1::uuid
		`, resourceID, hasName, name, hasLocation, location)
		if err != nil {
			return nil, fmt.Errorf("%s: update resources: %w", op, mapSQLError(err))
		}

		affectedRowsCount, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("%s: rows affected: %w", op, err)
		}
		if affectedRowsCount == 0 {
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
	}

	if hasDetails {
		err = updateDetailsByType(ctx, transaction, resourceID, in.Details)
		if err != nil {
			return nil, fmt.Errorf("%s: update details: %w", op, err)
		}
	}

	err = transaction.Commit()
	if err != nil {
		return nil, fmt.Errorf("%s: commit transaction: %w", op, err)
	}

	updatedResource, err := r.GetResource(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("%s: load updated resource: %w", op, err)
	}

	return updatedResource, nil
}

func (r *Repository) DeleteResource(ctx context.Context, resourceID string) error {
	const op = "postgres.DeleteResource"

	result, err := r.db.ExecContext(ctx, `
		DELETE FROM resources
		WHERE uuid = $1::uuid
	`, resourceID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	affectedRowsCount, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affectedRowsCount == 0 {
		return fmt.Errorf("%s: %w", op, models.ErrNotFound)
	}

	return nil
}

func (r *Repository) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	const op = "postgres.ChangeResourceStatus"

	if _, err := parseResourceStatus(string(status)); err != nil {
		return nil, fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
	}

	_ = reason

	result, err := r.db.ExecContext(ctx, `
		UPDATE resources
		SET status = $2, updated_at = now()
		WHERE uuid = $1::uuid
	`, resourceID, status)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	affectedRowsCount, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affectedRowsCount == 0 {
		return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
	}

	resource, err := r.GetResource(ctx, resourceID)
	if err != nil {
		return nil, fmt.Errorf("%s: load changed resource: %w", op, err)
	}

	return resource, nil
}

/////////////////////////////////////////////////////////////////////

func (r *Repository) queryResources(ctx context.Context, op, query string, args ...any) ([]*models.Resource, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}
	defer rows.Close()

	resources := make([]*models.Resource, 0)
	for rows.Next() {
		var scannedResource resourceRow
		err = rows.Scan(
			&scannedResource.id,
			&scannedResource.name,
			&scannedResource.typeRaw,
			&scannedResource.location,
			&scannedResource.statusRaw,
			&scannedResource.createdAt,
			&scannedResource.updatedAt,
			&scannedResource.mrCapacity,
			&scannedResource.mrHasProjector,
			&scannedResource.mrHasWhiteboard,
			&scannedResource.wsHasMonitor,
			&scannedResource.deviceType,
			&scannedResource.serialNumber,
			&scannedResource.model,
			&scannedResource.description,
		)
		if err != nil {
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

		resources = append(resources, resource)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	return resources, nil
}

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

func updateDetailsByType(ctx context.Context, tx *sql.Tx, resourceID string, details any) error {
	var (
		result sql.Result
		err    error
	)

	switch d := details.(type) {
	case *models.MeetingRoomDetails:
		if d == nil {
			return fmt.Errorf("%w: meeting_room details required", models.ErrInvalidType)
		}
		result, err = tx.ExecContext(ctx, `
			UPDATE meeting_rooms
			SET capacity = $1, has_projector = $2, has_whiteboard = $3
			WHERE resource_uuid = $4::uuid
		`, d.Capacity, d.HasProjector, d.HasWhiteboard, resourceID)
	case *models.WorkspaceDetails:
		if d == nil {
			return fmt.Errorf("%w: workspace details required", models.ErrInvalidType)
		}
		result, err = tx.ExecContext(ctx, `
			UPDATE workspaces
			SET has_monitor = $1
			WHERE resource_uuid = $2::uuid
		`, d.HasMonitor, resourceID)
	case *models.DeviceDetails:
		if d == nil {
			return fmt.Errorf("%w: device details required", models.ErrInvalidType)
		}
		result, err = tx.ExecContext(ctx, `
			UPDATE devices
			SET device_type = $1, serial_number = $2, model = $3, description = $4
			WHERE resource_uuid = $5::uuid
		`, d.DeviceType, nullableString(d.SerialNumber), nullableString(d.Model), nullableString(d.Description), resourceID)
	default:
		return models.ErrInvalidType
	}

	if err != nil {
		return mapSQLError(err)
	}

	affectedRowsCount, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affectedRowsCount == 0 {
		return models.ErrInvalidType
	}

	return nil
}
