//go:build integration

package platform

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	translationspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testTranslationPersistence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepo := coursespostgres.New(pool)
	store := courses.NewCourseVersionStore(courseRepo)
	course, err := courseRepo.CreateCourse(ctx, "translation-persistence")
	if err != nil {
		t.Fatal(err)
	}
	sourceOne := immutableCourseVersionFixture(t, course.ID, "1.0.0", "fe100000-0000-4000-8000-000000000001")
	storedOne, err := store.Store(ctx, sourceOne)
	if err != nil {
		t.Fatal(err)
	}
	creator, err := identitypostgres.New(pool).CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	service, err := translations.NewService(courseRepo, translationspostgres.New(pool), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	es, err := service.Create(ctx, storedOne.ID, "es", string(creator.ID))
	if err != nil {
		t.Fatal(err)
	}
	if es.Source.CourseVersionID != storedOne.ID || es.Source.Version.String() != "1.0.0" || es.Tree.Modules[0].SourceStableKey != storedOne.Modules[0].StableKey {
		t.Fatalf("source binding/tree not preserved: %#v", es)
	}
	if _, err := service.Create(ctx, storedOne.ID, "es", string(creator.ID)); !errors.Is(err, translations.ErrTranslationConflict) {
		t.Fatalf("duplicate source/language error = %v", err)
	}
	var wait sync.WaitGroup
	concurrent := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, createErr := service.Create(ctx, storedOne.ID, "fr", string(creator.ID))
			concurrent <- createErr
		}()
	}
	wait.Wait()
	close(concurrent)
	succeeded, conflicted := 0, 0
	for createErr := range concurrent {
		if createErr == nil {
			succeeded++
		} else if errors.Is(createErr, translations.ErrTranslationConflict) {
			conflicted++
		} else {
			t.Fatalf("concurrent translation creation: %v", createErr)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent uniqueness result: success=%d conflict=%d", succeeded, conflicted)
	}
	pl, err := service.Create(ctx, storedOne.ID, "pl", string(creator.ID))
	if err != nil {
		t.Fatal(err)
	}
	if pl.ID == es.ID || pl.Tree.Modules[0].Title != nil {
		t.Fatalf("cross-language workspaces were shared: %#v %#v", es, pl)
	}
	title := "Fundamentos"
	edited := es.Tree
	edited.Title = &title
	es, err = service.Update(ctx, es.ID, es.Revision, edited)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Publish(ctx, es.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != es.Revision || first.Tree.Title == nil || *first.Tree.Title != title {
		t.Fatalf("publication did not freeze current tree: %#v", first)
	}
	description := "Edited after publication"
	edited = es.Tree
	edited.Description = &description
	es, err = service.Update(ctx, es.ID, es.Revision, edited)
	if err != nil {
		t.Fatal(err)
	}
	if first.Tree.Description != nil {
		t.Fatalf("published snapshot changed after draft mutation: %#v", first)
	}
	second, err := service.Publish(ctx, es.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID || second.Revision != es.Revision {
		t.Fatalf("repeat publication did not create an immutable later snapshot: %#v %#v", first, second)
	}
	latest, err := service.LatestPublication(ctx, storedOne.ID, "es")
	if err != nil || latest.ID != second.ID {
		t.Fatalf("latest exact source/language publication = %#v err=%v", latest, err)
	}
	languages, err := service.Languages(ctx, storedOne.ID)
	if err != nil || len(languages) != 1 || languages[0] != "es" {
		t.Fatalf("published language list = %#v err=%v", languages, err)
	}

	// Publishing a newer source version never retargets or creates a translation.
	sourceTwo := immutableCourseVersionFixture(t, course.ID, "1.1.0", "fe100000-0000-4000-8000-000000000002")
	storedTwo, err := store.Store(ctx, sourceTwo)
	if err != nil {
		t.Fatal(err)
	}
	stillOne, err := service.Get(ctx, es.ID)
	if err != nil || stillOne.Source.CourseVersionID != storedOne.ID || stillOne.Source.Version.String() != "1.0.0" {
		t.Fatalf("new source version migrated existing translation: %#v err=%v", stillOne, err)
	}
	if _, err := service.GetBySourceVersionAndLanguage(ctx, storedTwo.ID, "es"); !errors.Is(err, translations.ErrTranslationNotFound) {
		t.Fatalf("new source version acquired translation automatically: %v", err)
	}
	esTwo, err := service.Create(ctx, storedTwo.ID, "es", string(creator.ID))
	if err != nil {
		t.Fatal(err)
	}
	if esTwo.ID == es.ID || esTwo.Source.CourseVersionID != storedTwo.ID {
		t.Fatalf("cross-version isolation failed: %#v %#v", es, esTwo)
	}
}
