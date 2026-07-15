package service

import (
	"context"
	"errors"
	"testing"

	"github.com/yang/wormhole_backend/internal/auth"
	"github.com/yang/wormhole_backend/internal/dal/model"
	"github.com/yang/wormhole_backend/internal/dto"
	"github.com/yang/wormhole_backend/internal/repository"
)

func TestResourceCategoryCreateRequiresAdmin(t *testing.T) {
	repo := &resourceRepoStub{}
	svc := NewResourceService(repo, &meRepo{
		roles: []model.Role{{Code: "user", Name: "普通用户"}},
	})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "alice"})

	_, err := svc.CreateCategory(ctx, dto.CreateResourceCategoryRequest{
		Code: "ai",
		Name: "AI 与机器学习",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateCategory error = %v, want ErrForbidden", err)
	}
	if repo.createdCategory != nil {
		t.Fatal("repository CreateCategory should not be called for a non-admin user")
	}
}

func TestResourceUpdateByAdminNormalizesEditableFields(t *testing.T) {
	categoryID := int32(3)
	name := " Claude "
	tags := []string{"ai", "ai", "", "assistant"}
	featured := false
	repo := &resourceRepoStub{
		category: &model.ResourceCategory{ID: categoryID, Code: "ai-ml", Name: "AI 与机器学习"},
		resource: repository.ResourceRecord{
			ID:         12,
			CategoryID: &categoryID,
			Slug:       "claude",
			Name:       "Claude",
		},
	}
	svc := NewResourceService(repo, &meRepo{
		roles: []model.Role{{Code: "admin", Name: "管理员"}},
	})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	_, err := svc.UpdateResource(ctx, 12, dto.UpdateResourceRequest{
		CategoryID: &categoryID,
		Name:       &name,
		Tags:       tags,
		Metadata:   &map[string]any{"tier": "enterprise"},
		IsFeatured: &featured,
	})
	if err != nil {
		t.Fatalf("UpdateResource returned error: %v", err)
	}
	if !repo.findCategoryCalled {
		t.Fatal("expected category existence check")
	}
	if got, _ := repo.resourceUpdates["name"].(string); got != "Claude" {
		t.Fatalf("name update = %q, want Claude", got)
	}
	if got, _ := repo.resourceUpdates["tags"].(string); got != `["ai","assistant"]` {
		t.Fatalf("tags update = %q, want normalized JSON array", got)
	}
	if got, _ := repo.resourceUpdates["metadata"].(string); got != `{"tier":"enterprise"}` {
		t.Fatalf("metadata update = %q, want JSON object", got)
	}
	if got, _ := repo.resourceUpdates["updated_by"].(int64); got != 7 {
		t.Fatalf("updated_by = %d, want 7", got)
	}
	if got, _ := repo.resourceUpdates["is_featured"].(bool); got {
		t.Fatalf("is_featured = %t, want false", got)
	}
}

func TestDeleteResourceCategoryByAdmin(t *testing.T) {
	repo := &resourceRepoStub{category: &model.ResourceCategory{ID: 3}}
	svc := NewResourceService(repo, &meRepo{roles: []model.Role{{Code: "admin", Name: "管理员"}}})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	if err := svc.DeleteCategory(ctx, 3); err != nil {
		t.Fatalf("DeleteCategory returned error: %v", err)
	}
	if repo.deletedCategoryID != 3 {
		t.Fatalf("deleted category ID = %d, want 3", repo.deletedCategoryID)
	}
}

func TestDeleteResourceByAdmin(t *testing.T) {
	repo := &resourceRepoStub{resource: repository.ResourceRecord{ID: 12}}
	svc := NewResourceService(repo, &meRepo{roles: []model.Role{{Code: "admin", Name: "管理员"}}})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "admin"})

	if err := svc.DeleteResource(ctx, 12); err != nil {
		t.Fatalf("DeleteResource returned error: %v", err)
	}
	if repo.deletedResourceID != 12 {
		t.Fatalf("deleted resource ID = %d, want 12", repo.deletedResourceID)
	}
}

func TestDeleteResourceRequiresAdmin(t *testing.T) {
	repo := &resourceRepoStub{resource: repository.ResourceRecord{ID: 12}}
	svc := NewResourceService(repo, &meRepo{roles: []model.Role{{Code: "user", Name: "普通用户"}}})
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7, Username: "user"})

	if err := svc.DeleteResource(ctx, 12); !errors.Is(err, ErrForbidden) {
		t.Fatalf("DeleteResource error = %v, want ErrForbidden", err)
	}
	if repo.deletedResourceID != 0 {
		t.Fatal("repository DeleteResource should not be called for a non-admin user")
	}
}

type resourceRepoStub struct {
	createdCategory    *model.ResourceCategory
	category           *model.ResourceCategory
	findCategoryCalled bool
	resource           repository.ResourceRecord
	resourceUpdates    map[string]any
	deletedCategoryID  int32
	deletedResourceID  int64
}

func (r *resourceRepoStub) ListCategories(ctx context.Context) ([]model.ResourceCategory, error) {
	return nil, nil
}

func (r *resourceRepoStub) AdminListCategories(ctx context.Context, status *int16) ([]model.ResourceCategory, error) {
	return nil, nil
}

func (r *resourceRepoStub) FindCategoryByID(ctx context.Context, id int32) (*model.ResourceCategory, error) {
	r.findCategoryCalled = true
	if r.category == nil || r.category.ID != id {
		return nil, repository.ErrResourceCategoryNotFound
	}
	return r.category, nil
}

func (r *resourceRepoStub) CreateCategory(ctx context.Context, category *model.ResourceCategory) error {
	category.ID = 1
	r.createdCategory = category
	return nil
}

func (r *resourceRepoStub) UpdateCategory(ctx context.Context, id int32, updates map[string]any) (*model.ResourceCategory, error) {
	if r.category == nil || r.category.ID != id {
		return nil, repository.ErrResourceCategoryNotFound
	}
	return r.category, nil
}

func (r *resourceRepoStub) UpdateCategorySortOrders(ctx context.Context, items []repository.ResourceCategorySortItem, updatedBy int64) error {
	return nil
}

func (r *resourceRepoStub) DeleteCategory(ctx context.Context, id int32) error {
	if r.category == nil || r.category.ID != id {
		return repository.ErrResourceCategoryNotFound
	}
	r.deletedCategoryID = id
	return nil
}

func (r *resourceRepoStub) ListResources(ctx context.Context, filter repository.ResourceFilter) ([]repository.ResourceRecord, int64, error) {
	return nil, 0, nil
}

func (r *resourceRepoStub) AdminListResources(ctx context.Context, filter repository.ResourceFilter) ([]repository.ResourceRecord, int64, error) {
	return nil, 0, nil
}

func (r *resourceRepoStub) FindResourceByID(ctx context.Context, id int64) (repository.ResourceRecord, error) {
	if r.resource.ID != id {
		return repository.ResourceRecord{}, repository.ErrResourceNotFound
	}
	return r.resource, nil
}

func (r *resourceRepoStub) FindResourceByIDAnyStatus(ctx context.Context, id int64) (repository.ResourceRecord, error) {
	if r.resource.ID != id {
		return repository.ResourceRecord{}, repository.ErrResourceNotFound
	}
	return r.resource, nil
}

func (r *resourceRepoStub) FindResourceBySlug(ctx context.Context, slug string) (repository.ResourceRecord, error) {
	if r.resource.Slug != slug {
		return repository.ResourceRecord{}, repository.ErrResourceNotFound
	}
	return r.resource, nil
}

func (r *resourceRepoStub) CreateResource(ctx context.Context, resource *model.Resource) error {
	resource.ID = 1
	r.resource.ID = resource.ID
	return nil
}

func (r *resourceRepoStub) UpdateResource(ctx context.Context, id int64, updates map[string]any) (repository.ResourceRecord, error) {
	if r.resource.ID != id {
		return repository.ResourceRecord{}, repository.ErrResourceNotFound
	}
	r.resourceUpdates = updates
	return r.resource, nil
}

func (r *resourceRepoStub) UpdateResourceSortOrders(ctx context.Context, items []repository.ResourceSortItem, updatedBy int64) error {
	return nil
}

func (r *resourceRepoStub) DeleteResource(ctx context.Context, id int64) error {
	if r.resource.ID != id {
		return repository.ErrResourceNotFound
	}
	r.deletedResourceID = id
	return nil
}

var _ repository.ResourceRepository = (*resourceRepoStub)(nil)
