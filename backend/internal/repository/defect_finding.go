package repository

import (
	"context"
	"strings"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"gorm.io/gorm"
)

// DefectFindingRepository owns all persistence operations for 缺陷发现.
type DefectFindingRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.DefectFinding], error)
	Get(context.Context, uint) (model.DefectFinding, error)
	Create(context.Context, *model.DefectFinding) error
	Update(context.Context, uint, uint, *model.DefectFinding) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	ListByFacility(context.Context, string) ([]model.DefectFinding, error)
}

type defectFindingRepository struct {
	store *Store[model.DefectFinding]
}

func NewDefectFindingRepository(db *gorm.DB) DefectFindingRepository {
	return &defectFindingRepository{store: NewStore[model.DefectFinding](db)}
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
func (r *defectFindingRepository) ListByFacility(ctx context.Context, facility string) ([]model.DefectFinding, error) {
	items := make([]model.DefectFinding, 0)
	err := r.store.DB().WithContext(ctx).
		Where("facility = ?", strings.TrimSpace(facility)).
		Order("code ASC").Find(&items).Error
	return items, err
}
