package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lib/pq"

	"github.com/acyushka/oregon-resource-service/internal/domain/models"
	service "github.com/acyushka/oregon-resource-service/internal/service/resource"
)

type Repository struct {
	db  *sql.DB
	log *slog.Logger
}

func New(ctx context.Context, dsn string, log *slog.Logger) (*Repository, error) {
	if log == nil {
		log = slog.Default()
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Error("postgres open failed", slog.Any("error", err))
		return nil, fmt.Errorf("postgres.New: %w", err)
	}

	if err = db.PingContext(ctx); err != nil {
		log.Error("postgres ping failed", slog.Any("error", err))
		if closeErr := db.Close(); closeErr != nil {
			log.Error("postgres close after ping failed", slog.Any("error", closeErr))
			return nil, fmt.Errorf("postgres.New: ping db: %w; close db: %v", err, closeErr)
		}
		return nil, fmt.Errorf("postgres.New: %w", err)
	}

	return &Repository{db: db, log: log}, nil
}

func (r *Repository) Close() error {
	err := r.db.Close()
	if err != nil {
		r.log.Error("postgres close failed", slog.Any("error", err))
	}

	return err
}

func (r *Repository) CreateResource(ctx context.Context, resource *models.Resource) (createdResource *models.Resource, err error) {
	const op = "postgres.CreateResource"

	if resource == nil {
		r.log.Error("create resource failed: nil resource")
		return nil, fmt.Errorf("%s: resource is nil", op)
	}

	transaction, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("create resource begin transaction failed", slog.Any("error", err))
		return nil, fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer func() {
		if err == nil {
			return
		}

		if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = fmt.Errorf("%w; rollback transaction: %v", err, rollbackErr)
		}
	}()

	createdResource = &models.Resource{}
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
		r.log.Error("create resource insert failed", slog.Any("error", err))
		return nil, fmt.Errorf("%s: insert resources: %w", op, mapSQLError(err))
	}

	if err = insertDetailsByType(ctx, transaction, createdResource.ID, createdResource.Type, resource.Details); err != nil {
		r.log.Error("create resource details insert failed", slog.String("resource_id", createdResource.ID), slog.Any("error", err))
		return nil, fmt.Errorf("%s: insert details: %w", op, err)
	}

	if err = transaction.Commit(); err != nil {
		r.log.Error("create resource commit failed", slog.String("resource_id", createdResource.ID), slog.Any("error", err))
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
			r.log.Warn("get resource not found", slog.String("resource_id", resourceID))
			return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
		}
		r.log.Error("get resource query failed", slog.String("resource_id", resourceID), slog.Any("error", err))
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

func (r *Repository) UpdateResource(ctx context.Context, resourceID string, in service.UpdateResourceRequest) (updatedResource *models.Resource, err error) {
	const op = "postgres.UpdateResource"

	transaction, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("update resource begin transaction failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return nil, fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer rollbackTxOnErr(transaction, &err)

	hasName, hasLocation, hasDetails, name, location := prepareUpdateResourceFields(in)

	if hasName || hasLocation || hasDetails {
		if err = updateResourceBaseFields(ctx, transaction, resourceID, hasName, name, hasLocation, location); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				r.log.Warn("update resource not found", slog.String("resource_id", resourceID))
			} else {
				r.log.Error("update resource base fields failed", slog.String("resource_id", resourceID), slog.Any("error", err))
			}

			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}

	if hasDetails {
		err = updateDetailsByType(ctx, transaction, resourceID, in.Details)
		if err != nil {
			r.log.Error("update resource details failed", slog.String("resource_id", resourceID), slog.Any("error", err))
			return nil, fmt.Errorf("%s: update details: %w", op, err)
		}
	}

	err = transaction.Commit()
	if err != nil {
		r.log.Error("update resource commit failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return nil, fmt.Errorf("%s: commit transaction: %w", op, err)
	}

	updatedResource, err = r.GetResource(ctx, resourceID)
	if err != nil {
		r.log.Error("update resource load updated failed", slog.String("resource_id", resourceID), slog.Any("error", err))
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
		r.log.Error("delete resource failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	affectedRowsCount, err := result.RowsAffected()
	if err != nil {
		r.log.Error("delete resource rows affected failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affectedRowsCount == 0 {
		r.log.Warn("delete resource not found", slog.String("resource_id", resourceID))
		return fmt.Errorf("%s: %w", op, models.ErrNotFound)
	}

	return nil
}

func (r *Repository) ChangeResourceStatus(ctx context.Context, resourceID string, status models.ResourceStatus, reason string) (*models.Resource, error) {
	const op = "postgres.ChangeResourceStatus"

	if _, err := parseResourceStatus(string(status)); err != nil {
		r.log.Warn("change status invalid status", slog.String("resource_id", resourceID), slog.String("status", string(status)))
		return nil, fmt.Errorf("%s: %w", op, models.ErrInvalidStatus)
	}

	_ = reason

	result, err := r.db.ExecContext(ctx, `
		UPDATE resources
		SET status = $2, updated_at = now()
		WHERE uuid = $1::uuid
	`, resourceID, status)
	if err != nil {
		r.log.Error("change status update failed", slog.String("resource_id", resourceID), slog.String("status", string(status)), slog.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	affectedRowsCount, err := result.RowsAffected()
	if err != nil {
		r.log.Error("change status rows affected failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return nil, fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affectedRowsCount == 0 {
		r.log.Warn("change status resource not found", slog.String("resource_id", resourceID))
		return nil, fmt.Errorf("%s: %w", op, models.ErrNotFound)
	}

	resource, err := r.GetResource(ctx, resourceID)
	if err != nil {
		r.log.Error("change status load changed resource failed", slog.String("resource_id", resourceID), slog.Any("error", err))
		return nil, fmt.Errorf("%s: load changed resource: %w", op, err)
	}

	return resource, nil
}

/////////////////////////////////////////////////////////////////////

func (r *Repository) queryResources(ctx context.Context, op, query string, args ...any) (resources []*models.Resource, err error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		r.log.Error("query resources failed", slog.String("op", op), slog.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			r.log.Error("query resources close rows failed", slog.String("op", op), slog.Any("error", closeErr))
			err = fmt.Errorf("%s: close rows: %w", op, closeErr)
		}
	}()

	resources = make([]*models.Resource, 0)
	for rows.Next() {
		scannedResource, scanErr := scanResourceRows(rows)
		if scanErr != nil {
			r.log.Error("query resources scan failed", slog.String("op", op), slog.Any("error", scanErr))
			return nil, fmt.Errorf("%s: %w", op, mapSQLError(scanErr))
		}

		resource, buildErr := buildResourceFromRow(scannedResource)
		if buildErr != nil {
			r.log.Error("query resources build model failed", slog.String("op", op), slog.Any("error", buildErr))
			return nil, fmt.Errorf("%s: %w", op, buildErr)
		}

		resources = append(resources, resource)
	}

	if err = rows.Err(); err != nil {
		r.log.Error("query resources iteration failed", slog.String("op", op), slog.Any("error", err))
		return nil, fmt.Errorf("%s: %w", op, mapSQLError(err))
	}

	return resources, nil
}

func rollbackTxOnErr(transaction *sql.Tx, operationErr *error) {
	if operationErr == nil || *operationErr == nil {
		return
	}

	if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		*operationErr = fmt.Errorf("%w; rollback transaction: %v", *operationErr, rollbackErr)
	}
}

func prepareUpdateResourceFields(in service.UpdateResourceRequest) (hasName bool, hasLocation bool, hasDetails bool, name any, location any) {
	hasName = in.Name != nil
	hasLocation = in.Location != nil
	hasDetails = in.Details != nil

	if hasName {
		name = *in.Name
	}
	if hasLocation {
		location = *in.Location
	}

	return hasName, hasLocation, hasDetails, name, location
}

func updateResourceBaseFields(
	ctx context.Context,
	transaction *sql.Tx,
	resourceID string,
	hasName bool,
	name any,
	hasLocation bool,
	location any,
) error {
	result, err := transaction.ExecContext(ctx, `
		UPDATE resources
		SET
			name = CASE WHEN $2 THEN $3 ELSE name END,
			location = CASE WHEN $4 THEN $5 ELSE location END,
			updated_at = now()
		WHERE uuid = $1::uuid
	`, resourceID, hasName, name, hasLocation, location)
	if err != nil {
		return fmt.Errorf("update resources: %w", mapSQLError(err))
	}

	affectedRowsCount, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affectedRowsCount == 0 {
		return models.ErrNotFound
	}

	return nil
}

func insertDetailsByType(ctx context.Context, tx *sql.Tx, resourceID string, resourceType models.ResourceType, details any) error {
	switch resourceType {
	case models.ResourceTypeMeetingRoom:
		return insertMeetingRoomDetails(ctx, tx, resourceID, details)
	case models.ResourceTypeWorkspace:
		return insertWorkspaceDetails(ctx, tx, resourceID, details)
	case models.ResourceTypeDevice:
		return insertDeviceDetails(ctx, tx, resourceID, details)
	default:
		return models.ErrInvalidType
	}
}

func insertMeetingRoomDetails(ctx context.Context, tx *sql.Tx, resourceID string, details any) error {
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
}

func insertWorkspaceDetails(ctx context.Context, tx *sql.Tx, resourceID string, details any) error {
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
}

func insertDeviceDetails(ctx context.Context, tx *sql.Tx, resourceID string, details any) error {
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
