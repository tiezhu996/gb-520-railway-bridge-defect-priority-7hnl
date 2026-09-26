package repository

import (
	"context"
	"strings"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/constants"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"gorm.io/gorm"
)

// FacilityKey normalizes the facility/bridge label used to correlate a
// priority decision with defects on the same bridge.
func FacilityKey(facility string) string {
	return strings.ToLower(strings.TrimSpace(facility))
}

// DefectFindingRepository owns all persistence operations for 缺陷发现.
type DefectFindingRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.DefectFinding], error)
	Get(context.Context, uint) (model.DefectFinding, error)
	Create(context.Context, *model.DefectFinding) error
	Update(context.Context, uint, uint, *model.DefectFinding) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	// OutstandingByFacility returns defects that are not mitigated or closed,
	// keyed by the (trimmed) facility/bridge they belong to. It backs the
	// automatic priority-decision release marker.
	OutstandingByFacility(context.Context) (map[string][]model.DefectFinding, error)
}

type defectFindingRepository struct {
	db    *gorm.DB
	store *Store[model.DefectFinding]
}

func NewDefectFindingRepository(db *gorm.DB) DefectFindingRepository {
	return &defectFindingRepository{db: db, store: NewStore[model.DefectFinding](db)}
}

func (r *defectFindingRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.DefectFinding], error) {
	return r.store.List(ctx, q)
}
func (r *defectFindingRepository) Get(ctx context.Context, id uint) (model.DefectFinding, error) {
	return r.store.Get(ctx, id)
}
func (r *defectFindingRepository) Create(ctx context.Context, item *model.DefectFinding) error {
	return r.store.Create(ctx, item)
}
func (r *defectFindingRepository) Update(ctx context.Context, id, version uint, item *model.DefectFinding) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *defectFindingRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *defectFindingRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}

func (r *defectFindingRepository) OutstandingByFacility(ctx context.Context) (map[string][]model.DefectFinding, error) {
	resolved := make([]string, 0, len(constants.ResolvedDefectStates))
	for status := range constants.ResolvedDefectStates {
		resolved = append(resolved, status)
	}
	var defects []model.DefectFinding
	err := r.db.WithContext(ctx).Model(&model.DefectFinding{}).
		Where("status NOT IN ?", resolved).
		Order("code ASC").
		Find(&defects).Error
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]model.DefectFinding)
	for _, defect := range defects {
		key := FacilityKey(defect.Facility)
		grouped[key] = append(grouped[key], defect)
	}
	return grouped, nil
}
