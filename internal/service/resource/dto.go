package resource

import "github.com/acyushka/oregon-resource-service/internal/domain/models"

type CreateResourceRequest struct {
	Name     string
	Type     models.ResourceType
	Location string
	Details  any
}

type UpdateResourceRequest struct {
	Name     *string
	Location *string
	Details  any
}
