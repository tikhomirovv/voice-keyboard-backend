package interfaces

import (
	"context"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
)

type ReleasesServiceInterface interface {
	GetReleases(ctx context.Context, options *dto.ReleasesOptions) ([]*dto.Release, error)
}
