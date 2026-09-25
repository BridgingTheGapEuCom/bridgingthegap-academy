package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDashboardPlacementNotFound = errors.New("dashboard widget placement not found")
	ErrDashboardPlacementConflict = errors.New("dashboard widget placement conflict")
)

// DashboardPlacement is installation-wide Dashboard composition. It is not a
// plugin release property and carries no trust state.
type DashboardPlacement struct {
	ID             string          `json:"placementId"`
	PluginID       PluginID        `json:"pluginId"`
	PluginVersion  string          `json:"pluginVersion"`
	ArtifactDigest string          `json:"artifactDigest"`
	WidgetID       string          `json:"widgetId"`
	Configuration  json.RawMessage `json:"configuration"`
	Position       int             `json:"position"`
	Enabled        bool            `json:"enabled"`
	Revision       int64           `json:"revision"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type DashboardPlacementRepository interface {
	ListDashboardPlacements(context.Context) ([]DashboardPlacement, error)
	CreateDashboardPlacement(context.Context, DashboardPlacement) (DashboardPlacement, error)
	UpdateDashboardPlacement(context.Context, DashboardPlacement, int64) (DashboardPlacement, error)
	DeleteDashboardPlacement(context.Context, string, int64) error
	ReorderDashboardPlacements(context.Context, []string, map[string]int64, time.Time) ([]DashboardPlacement, error)
}

type DashboardPlacementService struct {
	registry   *RegistryService
	repository DashboardPlacementRepository
	now        func() time.Time
}

func NewDashboardPlacementService(registry *RegistryService, repository DashboardPlacementRepository, now func() time.Time) *DashboardPlacementService {
	if now == nil {
		now = time.Now
	}
	return &DashboardPlacementService{registry, repository, now}
}
func (s *DashboardPlacementService) List(ctx context.Context) ([]DashboardPlacement, error) {
	if s == nil || s.repository == nil {
		return nil, ErrLaunchDenied
	}
	return s.repository.ListDashboardPlacements(ctx)
}
func (s *DashboardPlacementService) Create(ctx context.Context, value DashboardPlacement) (DashboardPlacement, error) {
	if s == nil || s.repository == nil || s.registry == nil {
		return DashboardPlacement{}, ErrLaunchDenied
	}
	value.ID = uuid.NewString()
	value.Position = -1
	value.Enabled = true
	value.Revision = 1
	value.CreatedAt = s.now().UTC()
	value.UpdatedAt = value.CreatedAt
	if err := s.validate(ctx, value); err != nil {
		return DashboardPlacement{}, err
	}
	return s.repository.CreateDashboardPlacement(ctx, value)
}
func (s *DashboardPlacementService) Update(ctx context.Context, value DashboardPlacement, expected int64) (DashboardPlacement, error) {
	if s == nil || s.repository == nil {
		return DashboardPlacement{}, ErrLaunchDenied
	}
	if err := s.validate(ctx, value); err != nil {
		return DashboardPlacement{}, err
	}
	value.UpdatedAt = s.now().UTC()
	return s.repository.UpdateDashboardPlacement(ctx, value, expected)
}
func (s *DashboardPlacementService) Delete(ctx context.Context, id string, expected int64) error {
	if s == nil || s.repository == nil || uuid.Validate(id) != nil || expected < 1 {
		return ErrDashboardPlacementNotFound
	}
	return s.repository.DeleteDashboardPlacement(ctx, id, expected)
}
func (s *DashboardPlacementService) Reorder(ctx context.Context, ids []string, revisions map[string]int64) ([]DashboardPlacement, error) {
	if s == nil || s.repository == nil {
		return nil, ErrLaunchDenied
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if uuid.Validate(id) != nil || seen[id] || revisions[id] < 1 {
			return nil, ErrDashboardPlacementConflict
		}
		seen[id] = true
	}
	return s.repository.ReorderDashboardPlacements(ctx, ids, revisions, s.now().UTC())
}
func (s *DashboardPlacementService) validate(ctx context.Context, value DashboardPlacement) error {
	if uuid.Validate(value.ID) != nil || value.Position < -1 || len(value.Configuration) == 0 || len(value.Configuration) > 16<<10 {
		return ErrLaunchDenied
	}
	var object map[string]json.RawMessage
	if strictDecode(value.Configuration, &object) != nil || object == nil {
		return ErrLaunchDenied
	}
	return s.validateRelease(ctx, value)
}
func (s *DashboardPlacementService) validateRelease(ctx context.Context, value DashboardPlacement) error {
	release, err := s.registry.RuntimeRelease(ctx, value.PluginID, value.PluginVersion)
	if err != nil || release.Release.ArtifactDigest != value.ArtifactDigest {
		return ErrLaunchDenied
	}
	for _, entry := range release.Manifest.Entrypoints {
		if entry.ID == value.WidgetID && entry.Type == TypeDashboardWidget {
			return nil
		}
	}
	return ErrLaunchDenied
}
