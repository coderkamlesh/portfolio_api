package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/apierr"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// skillTestNow is the fixed clock every skills test runs on.
var skillTestNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// fakeSkillStore is an in-memory SkillStore. It mirrors the real repository:
// reads return copies, misses are reported as models.ErrNotFound and name
// lookups ignore case.
type fakeSkillStore struct {
	categories []models.SkillCategory
	skills     []models.Skill
	// conflictOnWrite makes every write fail with models.ErrConflict, standing in
	// for the database UNIQUE index firing after a successful pre-check.
	conflictOnWrite bool
}

func (s *fakeSkillStore) ListCategories(context.Context) ([]models.SkillCategory, error) {
	out := append([]models.SkillCategory(nil), s.categories...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *fakeSkillStore) FindCategoryByID(_ context.Context, id string) (*models.SkillCategory, error) {
	for i := range s.categories {
		if s.categories[i].ID == id {
			category := s.categories[i]
			return &category, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeSkillStore) FindCategoryByName(_ context.Context, name string) (*models.SkillCategory, error) {
	for i := range s.categories {
		if strings.EqualFold(s.categories[i].Name, name) {
			category := s.categories[i]
			return &category, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeSkillStore) CreateCategory(_ context.Context, category *models.SkillCategory) error {
	if s.conflictOnWrite {
		return models.ErrConflict
	}
	s.categories = append(s.categories, *category)
	return nil
}

func (s *fakeSkillStore) UpdateCategory(_ context.Context, category *models.SkillCategory) error {
	if s.conflictOnWrite {
		return models.ErrConflict
	}
	for i := range s.categories {
		if s.categories[i].ID == category.ID {
			s.categories[i] = *category
			return nil
		}
	}
	return models.ErrNotFound
}

// DeleteCategory also drops the skills of the category, exactly like the
// transactional repository does.
func (s *fakeSkillStore) DeleteCategory(_ context.Context, id string) error {
	index := -1
	for i := range s.categories {
		if s.categories[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return models.ErrNotFound
	}
	s.categories = append(s.categories[:index], s.categories[index+1:]...)

	remaining := make([]models.Skill, 0, len(s.skills))
	for i := range s.skills {
		if s.skills[i].CategoryID != id {
			remaining = append(remaining, s.skills[i])
		}
	}
	s.skills = remaining
	return nil
}

func (s *fakeSkillStore) ListSkills(_ context.Context, categoryID string) ([]models.Skill, error) {
	out := make([]models.Skill, 0, len(s.skills))
	for i := range s.skills {
		if categoryID == "" || s.skills[i].CategoryID == categoryID {
			out = append(out, s.skills[i])
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DisplayOrder != out[j].DisplayOrder {
			return out[i].DisplayOrder < out[j].DisplayOrder
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *fakeSkillStore) FindSkillByID(_ context.Context, id string) (*models.Skill, error) {
	for i := range s.skills {
		if s.skills[i].ID == id {
			skill := s.skills[i]
			return &skill, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeSkillStore) FindSkillByName(_ context.Context, categoryID, name string) (*models.Skill, error) {
	for i := range s.skills {
		if s.skills[i].CategoryID == categoryID && strings.EqualFold(s.skills[i].Name, name) {
			skill := s.skills[i]
			return &skill, nil
		}
	}
	return nil, models.ErrNotFound
}

func (s *fakeSkillStore) CreateSkill(_ context.Context, skill *models.Skill) error {
	if s.conflictOnWrite {
		return models.ErrConflict
	}
	s.skills = append(s.skills, *skill)
	return nil
}

func (s *fakeSkillStore) UpdateSkill(_ context.Context, skill *models.Skill) error {
	if s.conflictOnWrite {
		return models.ErrConflict
	}
	for i := range s.skills {
		if s.skills[i].ID == skill.ID {
			s.skills[i] = *skill
			return nil
		}
	}
	return models.ErrNotFound
}

func (s *fakeSkillStore) DeleteSkill(_ context.Context, id string) error {
	for i := range s.skills {
		if s.skills[i].ID == id {
			s.skills = append(s.skills[:i], s.skills[i+1:]...)
			return nil
		}
	}
	return models.ErrNotFound
}

// newSkillTestService builds the service over a fixed clock.
func newSkillTestService(store *fakeSkillStore) *SkillService {
	return NewSkillService(SkillDeps{Skills: store, Now: func() time.Time { return skillTestNow }})
}

// skillErrCode returns the API error code of err, failing the test when err is
// nil.
func skillErrCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return apierr.From(err).Code
}

// intPtr is the pointer the optional display_order payload field decodes into.
func intPtr(v int) *int { return &v }

func TestPublicSkillsGroupsSkillsByCategory(t *testing.T) {
	store := &fakeSkillStore{
		categories: []models.SkillCategory{
			{ID: "cat-lang", Name: "Languages", DisplayOrder: 1},
			{ID: "cat-cloud", Name: "Cloud", DisplayOrder: 2},
		},
		skills: []models.Skill{
			{ID: "s-java", CategoryID: "cat-lang", Name: "Java", DisplayOrder: 2},
			{ID: "s-go", CategoryID: "cat-lang", Name: "Go", DisplayOrder: 1},
		},
	}
	svc := newSkillTestService(store)

	got, err := svc.PublicSkills(context.Background())
	if err != nil {
		t.Fatalf("PublicSkills: %v", err)
	}
	if len(got.Categories) != 2 {
		t.Fatalf("got %d categories, want 2: %+v", len(got.Categories), got.Categories)
	}

	first := got.Categories[0]
	if first.Name != "Languages" || len(first.Skills) != 2 {
		t.Fatalf("first category = %+v, want Languages with 2 skills", first)
	}
	if first.Skills[0].Name != "Go" || first.Skills[1].Name != "Java" {
		t.Errorf("skills were not ordered by display_order: %+v", first.Skills)
	}

	second := got.Categories[1]
	if second.Name != "Cloud" {
		t.Errorf("second category = %q, want Cloud", second.Name)
	}
	if second.Skills == nil || len(second.Skills) != 0 {
		t.Errorf("an empty category must render as [], got %#v", second.Skills)
	}
}

func TestPublicSkillsReturnsEmptyCategoriesWhenNothingConfigured(t *testing.T) {
	svc := newSkillTestService(&fakeSkillStore{})

	got, err := svc.PublicSkills(context.Background())
	if err != nil {
		t.Fatalf("PublicSkills: %v", err)
	}
	if got.Categories == nil || len(got.Categories) != 0 {
		t.Fatalf("Categories = %#v, want an empty non-nil slice", got.Categories)
	}
}

func TestAdminListsAreEmptySlicesNotNil(t *testing.T) {
	svc := newSkillTestService(&fakeSkillStore{})

	categories, err := svc.AdminCategories(context.Background())
	if err != nil {
		t.Fatalf("AdminCategories: %v", err)
	}
	if categories.Categories == nil || len(categories.Categories) != 0 {
		t.Errorf("Categories = %#v, want an empty non-nil slice", categories.Categories)
	}

	skills, err := svc.AdminSkills(context.Background())
	if err != nil {
		t.Fatalf("AdminSkills: %v", err)
	}
	if skills.Skills == nil || len(skills.Skills) != 0 {
		t.Errorf("Skills = %#v, want an empty non-nil slice", skills.Skills)
	}
}

func TestCreateCategoryTrimsNameAndDefaultsDisplayOrder(t *testing.T) {
	store := &fakeSkillStore{}
	svc := newSkillTestService(store)

	got, err := svc.CreateCategory(context.Background(), CategoryInput{Name: "  Backend  "})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if got.Name != "Backend" {
		t.Errorf("Name = %q, want Backend", got.Name)
	}
	if got.DisplayOrder != 0 {
		t.Errorf("DisplayOrder = %d, want 0 when the payload omits it", got.DisplayOrder)
	}
	if got.ID == "" {
		t.Error("a new category must have an ID")
	}
	if !got.CreatedAt.Equal(skillTestNow) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, skillTestNow)
	}
	if len(store.categories) != 1 {
		t.Fatalf("stored %d categories, want 1", len(store.categories))
	}
}

func TestCreateCategoryStoresExplicitDisplayOrder(t *testing.T) {
	store := &fakeSkillStore{}
	svc := newSkillTestService(store)

	got, err := svc.CreateCategory(context.Background(), CategoryInput{Name: "Cloud", DisplayOrder: intPtr(3)})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if got.DisplayOrder != 3 || store.categories[0].DisplayOrder != 3 {
		t.Errorf("DisplayOrder was not stored: view=%d stored=%d", got.DisplayOrder, store.categories[0].DisplayOrder)
	}
}

func TestCreateCategoryRejectsDuplicateNameCaseInsensitively(t *testing.T) {
	store := &fakeSkillStore{categories: []models.SkillCategory{{ID: "c1", Name: "Backend"}}}
	svc := newSkillTestService(store)

	_, err := svc.CreateCategory(context.Background(), CategoryInput{Name: " backend "})
	if code := skillErrCode(t, err); code != CodeSkillCategoryExists {
		t.Fatalf("CreateCategory returned %v (code %q), want %q", err, code, CodeSkillCategoryExists)
	}
	if len(store.categories) != 1 {
		t.Errorf("a duplicate must not be stored: %+v", store.categories)
	}
}

func TestCreateCategoryRequiresName(t *testing.T) {
	store := &fakeSkillStore{}
	svc := newSkillTestService(store)

	_, err := svc.CreateCategory(context.Background(), CategoryInput{Name: "   "})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("CreateCategory returned %v (code %q), want validation_failed", err, code)
	}
	if len(store.categories) != 0 {
		t.Errorf("invalid input must not be stored: %+v", store.categories)
	}
}

func TestUpdateCategoryKeepsDisplayOrderWhenOmittedAndAppliesItWhenGiven(t *testing.T) {
	store := &fakeSkillStore{categories: []models.SkillCategory{{ID: "c1", Name: "Backend", DisplayOrder: 7}}}
	svc := newSkillTestService(store)

	renamed, err := svc.UpdateCategory(context.Background(), "c1", CategoryInput{Name: "Backend Engineering"})
	if err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}
	if renamed.Name != "Backend Engineering" {
		t.Errorf("Name = %q", renamed.Name)
	}
	if renamed.DisplayOrder != 7 {
		t.Errorf("DisplayOrder = %d, want the stored 7 when the payload omits it", renamed.DisplayOrder)
	}

	reordered, err := svc.UpdateCategory(context.Background(), "c1", CategoryInput{Name: "Backend Engineering", DisplayOrder: intPtr(2)})
	if err != nil {
		t.Fatalf("UpdateCategory: %v", err)
	}
	if reordered.DisplayOrder != 2 || store.categories[0].DisplayOrder != 2 {
		t.Errorf("DisplayOrder was not applied: view=%d stored=%d", reordered.DisplayOrder, store.categories[0].DisplayOrder)
	}
}

func TestUpdateCategoryRejectsNameTakenByAnotherCategory(t *testing.T) {
	store := &fakeSkillStore{categories: []models.SkillCategory{
		{ID: "c1", Name: "Backend"},
		{ID: "c2", Name: "Frontend"},
	}}
	svc := newSkillTestService(store)

	_, err := svc.UpdateCategory(context.Background(), "c1", CategoryInput{Name: "frontend"})
	if code := skillErrCode(t, err); code != CodeSkillCategoryExists {
		t.Fatalf("UpdateCategory returned %v (code %q), want %q", err, code, CodeSkillCategoryExists)
	}
	if store.categories[0].Name != "Backend" {
		t.Errorf("a rejected rename must not be persisted: %+v", store.categories[0])
	}
}

func TestUpdateCategoryNotFound(t *testing.T) {
	svc := newSkillTestService(&fakeSkillStore{})

	_, err := svc.UpdateCategory(context.Background(), "missing", CategoryInput{Name: "Backend"})
	if code := skillErrCode(t, err); code != CodeSkillCategoryNotFound {
		t.Fatalf("UpdateCategory returned %v (code %q), want %q", err, code, CodeSkillCategoryNotFound)
	}
}

func TestDeleteCategoryRemovesItsSkills(t *testing.T) {
	store := &fakeSkillStore{
		categories: []models.SkillCategory{{ID: "c1", Name: "Backend"}, {ID: "c2", Name: "Cloud"}},
		skills: []models.Skill{
			{ID: "s1", CategoryID: "c1", Name: "Go"},
			{ID: "s2", CategoryID: "c2", Name: "AWS"},
		},
	}
	svc := newSkillTestService(store)

	if err := svc.DeleteCategory(context.Background(), "c1"); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	if len(store.categories) != 1 || store.categories[0].ID != "c2" {
		t.Errorf("categories after delete = %+v", store.categories)
	}
	if len(store.skills) != 1 || store.skills[0].ID != "s2" {
		t.Errorf("skills of the deleted category must be gone: %+v", store.skills)
	}
}

func TestDeleteCategoryNotFound(t *testing.T) {
	svc := newSkillTestService(&fakeSkillStore{})

	if err := svc.DeleteCategory(context.Background(), "missing"); skillErrCode(t, err) != CodeSkillCategoryNotFound {
		t.Fatalf("DeleteCategory returned %v, want %q", err, CodeSkillCategoryNotFound)
	}
}

// skillTestStore returns a store holding the categories "Backend" and "Cloud".
func skillTestStore() *fakeSkillStore {
	return &fakeSkillStore{
		categories: []models.SkillCategory{
			{ID: "c1", Name: "Backend", DisplayOrder: 1, CreatedAt: skillTestNow},
			{ID: "c2", Name: "Cloud", DisplayOrder: 2, CreatedAt: skillTestNow},
		},
	}
}

func TestCreateSkillValidatesPayload(t *testing.T) {
	store := skillTestStore()
	svc := newSkillTestService(store)

	_, err := svc.CreateSkill(context.Background(), SkillInput{Name: "Go"})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("missing category_id returned %v (code %q)", err, code)
	}

	_, err = svc.CreateSkill(context.Background(), SkillInput{CategoryID: "c1"})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("missing name returned %v (code %q)", err, code)
	}

	if len(store.skills) != 0 {
		t.Errorf("invalid input must not be stored: %+v", store.skills)
	}
}

func TestCreateSkillRequiresExistingCategory(t *testing.T) {
	store := skillTestStore()
	svc := newSkillTestService(store)

	_, err := svc.CreateSkill(context.Background(), SkillInput{CategoryID: "missing", Name: "Go"})
	if code := skillErrCode(t, err); code != CodeSkillCategoryNotFound {
		t.Fatalf("CreateSkill returned %v (code %q), want %q", err, code, CodeSkillCategoryNotFound)
	}
	if len(store.skills) != 0 {
		t.Errorf("orphan skill must not be stored: %+v", store.skills)
	}
}

func TestCreateSkillTrimsFieldsAndDefaultsDisplayOrder(t *testing.T) {
	store := skillTestStore()
	svc := newSkillTestService(store)

	got, err := svc.CreateSkill(context.Background(), SkillInput{
		CategoryID: " c1 ",
		Name:       "  Go  ",
		IconSlug:   "  go  ",
	})
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	if got.Name != "Go" || got.IconSlug != "go" || got.CategoryID != "c1" {
		t.Errorf("skill was not normalized: %+v", got)
	}
	if got.DisplayOrder != 0 {
		t.Errorf("DisplayOrder = %d, want 0 when the payload omits it", got.DisplayOrder)
	}
	if got.ID == "" || !got.CreatedAt.Equal(skillTestNow) {
		t.Errorf("created metadata missing: %+v", got)
	}
}

func TestCreateSkillRejectsDuplicateInsideTheSameCategory(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "Go"}}
	svc := newSkillTestService(store)

	_, err := svc.CreateSkill(context.Background(), SkillInput{CategoryID: "c1", Name: " go "})
	if code := skillErrCode(t, err); code != CodeSkillExists {
		t.Fatalf("CreateSkill returned %v (code %q), want %q", err, code, CodeSkillExists)
	}
	if len(store.skills) != 1 {
		t.Errorf("duplicate must not be stored: %+v", store.skills)
	}
}

func TestCreateSkillAllowsTheSameNameInAnotherCategory(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "Go"}}
	svc := newSkillTestService(store)

	got, err := svc.CreateSkill(context.Background(), SkillInput{CategoryID: "c2", Name: "Go"})
	if err != nil {
		t.Fatalf("same name in another category must be allowed: %v", err)
	}
	if got.CategoryID != "c2" || len(store.skills) != 2 {
		t.Errorf("skill was not stored in c2: %+v", got)
	}
}

func TestUpdateSkillMovesBetweenCategoriesAndKeepsDisplayOrder(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "Terraform", DisplayOrder: 4}}
	svc := newSkillTestService(store)

	got, err := svc.UpdateSkill(context.Background(), "s1", SkillInput{CategoryID: "c2", Name: "Terraform", IconSlug: "tf"})
	if err != nil {
		t.Fatalf("UpdateSkill: %v", err)
	}
	if got.CategoryID != "c2" || store.skills[0].CategoryID != "c2" {
		t.Errorf("skill was not moved: view=%+v stored=%+v", got, store.skills[0])
	}
	if got.DisplayOrder != 4 {
		t.Errorf("DisplayOrder = %d, want the stored 4 when the payload omits it", got.DisplayOrder)
	}
	if got.IconSlug != "tf" {
		t.Errorf("IconSlug = %q, want tf", got.IconSlug)
	}
}

func TestUpdateSkillRequiresNameAndKnownTargetCategory(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "Go"}}
	svc := newSkillTestService(store)

	_, err := svc.UpdateSkill(context.Background(), "s1", SkillInput{Name: "   "})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("blank name returned %v (code %q)", err, code)
	}

	_, err = svc.UpdateSkill(context.Background(), "s1", SkillInput{CategoryID: "missing", Name: "Go"})
	if code := skillErrCode(t, err); code != CodeSkillCategoryNotFound {
		t.Fatalf("unknown target category returned %v (code %q)", err, code)
	}
	if store.skills[0].CategoryID != "c1" {
		t.Errorf("a rejected move must not be persisted: %+v", store.skills[0])
	}
}

func TestUpdateSkillRejectsDuplicateNameInTargetCategory(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{
		{ID: "s1", CategoryID: "c1", Name: "Go"},
		{ID: "s2", CategoryID: "c2", Name: "Go"},
	}
	svc := newSkillTestService(store)

	_, err := svc.UpdateSkill(context.Background(), "s1", SkillInput{CategoryID: "c2", Name: "Go"})
	if code := skillErrCode(t, err); code != CodeSkillExists {
		t.Fatalf("UpdateSkill returned %v (code %q), want %q", err, code, CodeSkillExists)
	}
}

func TestUpdateSkillRenamingToADifferentCaseIsAllowed(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "go"}}
	svc := newSkillTestService(store)

	got, err := svc.UpdateSkill(context.Background(), "s1", SkillInput{Name: "Go"})
	if err != nil {
		t.Fatalf("renaming the same skill must be allowed: %v", err)
	}
	if got.Name != "Go" || store.skills[0].Name != "Go" {
		t.Errorf("rename was not applied: view=%+v stored=%+v", got, store.skills[0])
	}
}

func TestUpdateSkillNotFound(t *testing.T) {
	svc := newSkillTestService(skillTestStore())

	_, err := svc.UpdateSkill(context.Background(), "missing", SkillInput{Name: "Go"})
	if code := skillErrCode(t, err); code != CodeSkillNotFound {
		t.Fatalf("UpdateSkill returned %v (code %q), want %q", err, code, CodeSkillNotFound)
	}
}

func TestDeleteSkillRemovesOneRowAndReportsMissingOnes(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "Go"}}
	svc := newSkillTestService(store)

	if err := svc.DeleteSkill(context.Background(), "s1"); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}
	if len(store.skills) != 0 {
		t.Errorf("skill was not deleted: %+v", store.skills)
	}
	if err := svc.DeleteSkill(context.Background(), "s1"); skillErrCode(t, err) != CodeSkillNotFound {
		t.Fatalf("second delete returned %v, want %q", err, CodeSkillNotFound)
	}
}

func TestAdminSkillsExposeCategoryAndCreatedAt(t *testing.T) {
	store := skillTestStore()
	store.skills = []models.Skill{
		{ID: "s2", CategoryID: "c2", Name: "AWS", DisplayOrder: 2, CreatedAt: skillTestNow},
		{ID: "s1", CategoryID: "c1", Name: "Go", DisplayOrder: 1, CreatedAt: skillTestNow},
	}
	svc := newSkillTestService(store)

	got, err := svc.AdminSkills(context.Background())
	if err != nil {
		t.Fatalf("AdminSkills: %v", err)
	}
	if len(got.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(got.Skills))
	}
	if got.Skills[0].ID != "s1" || got.Skills[0].CategoryID != "c1" {
		t.Errorf("skills are not ordered by category: %+v", got.Skills)
	}
	if !got.Skills[0].CreatedAt.Equal(skillTestNow) {
		t.Errorf("CreatedAt = %v, want %v", got.Skills[0].CreatedAt, skillTestNow)
	}
}

func TestAdminCategoriesExposeCreatedAt(t *testing.T) {
	svc := newSkillTestService(skillTestStore())

	got, err := svc.AdminCategories(context.Background())
	if err != nil {
		t.Fatalf("AdminCategories: %v", err)
	}
	if len(got.Categories) != 2 {
		t.Fatalf("got %d categories, want 2", len(got.Categories))
	}
	if got.Categories[0].ID != "c1" || !got.Categories[0].CreatedAt.Equal(skillTestNow) {
		t.Errorf("first category = %+v", got.Categories[0])
	}
}

func TestCreateCategoryRejectsOverLongName(t *testing.T) {
	store := &fakeSkillStore{}
	svc := newSkillTestService(store)

	_, err := svc.CreateCategory(context.Background(), CategoryInput{Name: strings.Repeat("a", maxCategoryNameLen+1)})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("CreateCategory returned %v (code %q), want validation_failed", err, code)
	}
	if len(store.categories) != 0 {
		t.Errorf("an over-long name must not be stored: %+v", store.categories)
	}
}

func TestValidationRejectsControlCharacters(t *testing.T) {
	store := &fakeSkillStore{}
	svc := newSkillTestService(store)

	_, err := svc.CreateCategory(context.Background(), CategoryInput{Name: "Back\u0000end"})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("control characters returned %v (code %q), want validation_failed", err, code)
	}
	if len(store.categories) != 0 {
		t.Errorf("control characters must not be stored: %+v", store.categories)
	}
}

func TestValidationRejectsNegativeDisplayOrder(t *testing.T) {
	store := skillTestStore()
	svc := newSkillTestService(store)

	_, err := svc.CreateCategory(context.Background(), CategoryInput{Name: "Platform", DisplayOrder: intPtr(-1)})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("negative category order returned %v (code %q)", err, code)
	}

	_, err = svc.CreateSkill(context.Background(), SkillInput{CategoryID: "c1", Name: "Go", DisplayOrder: intPtr(-1)})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("negative skill order returned %v (code %q)", err, code)
	}
	if len(store.categories) != 2 || len(store.skills) != 0 {
		t.Errorf("rejected payloads must not be stored: %+v / %+v", store.categories, store.skills)
	}
}

func TestSkillRejectsOverLongNameAndIconSlugButAcceptsTheLimits(t *testing.T) {
	store := skillTestStore()
	svc := newSkillTestService(store)

	_, err := svc.CreateSkill(context.Background(), SkillInput{CategoryID: "c1", Name: strings.Repeat("n", maxSkillNameLen+1)})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("over-long skill name returned %v (code %q)", err, code)
	}

	_, err = svc.CreateSkill(context.Background(), SkillInput{
		CategoryID: "c1",
		Name:       "Go",
		IconSlug:   strings.Repeat("i", maxIconSlugLen+1),
	})
	if code := skillErrCode(t, err); code != "validation_failed" {
		t.Fatalf("over-long icon_slug returned %v (code %q)", err, code)
	}

	got, err := svc.CreateSkill(context.Background(), SkillInput{
		CategoryID: "c1",
		Name:       strings.Repeat("n", maxSkillNameLen),
		IconSlug:   strings.Repeat("i", maxIconSlugLen),
	})
	if err != nil {
		t.Fatalf("values exactly at the limit must be accepted: %v", err)
	}
	if len(got.IconSlug) != maxIconSlugLen {
		t.Errorf("icon_slug was truncated: %d characters", len(got.IconSlug))
	}
}

func TestUniqueViolationIsReportedAsConflict(t *testing.T) {
	// Every case uses a name that is not stored yet, so the service-level
	// duplicate check passes and the 409 can only come from the write failing
	// with models.ErrConflict — the safety net for a check/insert race.
	cases := []struct {
		name string
		call func(*SkillService) error
		want string
	}{
		{
			name: "create category",
			call: func(svc *SkillService) error {
				_, err := svc.CreateCategory(context.Background(), CategoryInput{Name: "Platform Engineering"})
				return err
			},
			want: CodeSkillCategoryExists,
		},
		{
			name: "update category",
			call: func(svc *SkillService) error {
				_, err := svc.UpdateCategory(context.Background(), "c1", CategoryInput{Name: "Backend Platform"})
				return err
			},
			want: CodeSkillCategoryExists,
		},
		{
			name: "create skill",
			call: func(svc *SkillService) error {
				_, err := svc.CreateSkill(context.Background(), SkillInput{CategoryID: "c1", Name: "Terraform"})
				return err
			},
			want: CodeSkillExists,
		},
		{
			name: "update skill",
			call: func(svc *SkillService) error {
				_, err := svc.UpdateSkill(context.Background(), "s1", SkillInput{Name: "Golang"})
				return err
			},
			want: CodeSkillExists,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := skillTestStore()
			store.skills = []models.Skill{{ID: "s1", CategoryID: "c1", Name: "Go"}}
			store.conflictOnWrite = true
			svc := newSkillTestService(store)

			err := tc.call(svc)
			if code := skillErrCode(t, err); code != tc.want {
				t.Fatalf("got %v (code %q), want %q", err, code, tc.want)
			}
		})
	}
}

