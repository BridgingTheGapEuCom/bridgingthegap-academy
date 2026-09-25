package plugins

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// DashboardWidgetLaunchService resolves the installation-owned placement.
// Callers provide only its opaque ID; plugin coordinates and configuration are
// always read from authoritative placement state.
type DashboardWidgetLaunchService struct {
	placements DashboardPlacementReader
	runtime    *RuntimeService
}

func NewDashboardWidgetLaunchService(placements DashboardPlacementReader, runtime *RuntimeService) *DashboardWidgetLaunchService {
	service := &DashboardWidgetLaunchService{placements: placements, runtime: runtime}
	if runtime != nil {
		runtime.dashboardPlacements = placements
	}
	return service
}

func (s *DashboardWidgetLaunchService) Prepare(ctx context.Context, placementID string) (RuntimeLaunch, error) {
	if s == nil || s.placements == nil || s.runtime == nil || uuid.Validate(placementID) != nil {
		return RuntimeLaunch{}, ErrLaunchDenied
	}
	placement, err := s.placements.GetDashboardPlacement(ctx, placementID)
	if errors.Is(err, ErrDashboardPlacementNotFound) || errors.Is(err, ErrNotFound) {
		return RuntimeLaunch{}, ErrLaunchDenied
	}
	if err != nil {
		return RuntimeLaunch{}, err
	}
	if placement.ID != placementID || !placement.Enabled {
		return RuntimeLaunch{}, ErrLaunchDenied
	}
	return s.runtime.prepareDashboardWidgetRuntime(ctx, placement)
}

func IsDashboardWidgetLaunchUnavailable(err error) bool {
	return errors.Is(err, ErrLaunchDenied) || errors.Is(err, ErrDashboardPlacementNotFound) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrResourceInvalid)
}
