//go:build integration

package platform

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/db/sqlc"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
	translationspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations/postgres"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testImportedTranslationPublication(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	courseRepo := coursespostgres.New(pool)
	translationRepo := translationspostgres.New(pool)
	course, err := courseRepo.CreateCourse(ctx, "imported-translation-publication")
	if err != nil {
		t.Fatal(err)
	}
	importedAt := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	record, err := sqlc.New(pool).CreateImportRecord(ctx, sqlc.CreateImportRecordParams{
		PackageFingerprint:     "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		PortableSourceCourseID: "portable-translated-course", PortableSourceCourseVersionID: "portable-translated-v1",
		SourceVersion: "1.0.0", ImportedAt: pgtype.Timestamptz{Time: importedAt, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := immutableCourseVersionFixture(t, course.ID, "1.0.0", "fe100000-0000-4000-8000-000000000099")
	input.Provenance = courses.CourseVersionProvenance{}
	input.Publication = courses.PublicationProvenance{Origin: courses.PublicationOriginImported, Imported: &courses.ImportedPublicationProvenance{
		ImportID: record.ID.String(), PackageFingerprint: record.PackageFingerprint, PortableSourceCourseID: record.PortableSourceCourseID,
		PortableSourceCourseVersionID: record.PortableSourceCourseVersionID, SourceVersion: input.CourseVersion.Version, ImportedAt: importedAt,
	}}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := courseRepo.StoreImmutableCourseVersionInTx(ctx, tx, input)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	tree, err := translations.SeedTranslationTree(stored)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	title, description := "Curso importado", "Descripción importada"
	completeImportedTranslation(&tree, title, description)
	provenance := translations.ImportedPublicationProvenance{ImportID: record.ID.String(), PackageFingerprint: record.PackageFingerprint, PortableSourceCourseID: record.PortableSourceCourseID, PortableSourceCourseVersionID: record.PortableSourceCourseVersionID, ImportedAt: importedAt}
	publication, err := translationRepo.StoreImportedPublicationInTx(ctx, tx, stored, "es", tree, provenance, importedAt)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if publication.Provenance.Origin != translations.PublicationOriginImported || publication.TranslationID != "" || publication.Revision != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("imported publication provenance = %#v", publication)
	}
	var inside, outside int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM translations.translation_publication WHERE id=$1`, publication.ID).Scan(&inside); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM translations.translation_publication WHERE id=$1`, publication.ID).Scan(&outside); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if inside != 1 || outside != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("imported publication visibility inside=%d outside=%d", inside, outside)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	duplicateTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := translationRepo.StoreImportedPublicationInTx(ctx, duplicateTx, stored, "es", tree, provenance, importedAt); !errors.Is(err, translations.ErrTranslationConflict) {
		_ = duplicateTx.Rollback(ctx)
		t.Fatalf("duplicate imported publication error = %v", err)
	}
	if err := duplicateTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	service, err := translations.NewService(courseRepo, translationRepo, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := service.LatestPublication(ctx, stored.ID, "es")
	if err != nil || latest.ID != publication.ID || latest.Provenance.Imported == nil || latest.Provenance.Imported.ImportID != record.ID.String() {
		t.Fatalf("latest imported publication = %#v, %v", latest, err)
	}
	languages, err := service.Languages(ctx, stored.ID)
	if err != nil || len(languages) != 1 || languages[0] != "es" {
		t.Fatalf("imported language discovery = %#v, %v", languages, err)
	}
	reader, err := translations.NewLearnerReader(service, courses.NewPublishedReadService(courseRepo))
	if err != nil {
		t.Fatal(err)
	}
	learner, err := reader.Read(ctx, course.ID, stored.CourseVersion.Version, "es")
	if err != nil || learner.Course.Title != title || learner.Course.Description != description || learner.Course.CourseID != course.ID || learner.Translation.PublicationID != publication.ID {
		t.Fatalf("imported learner read = %#v, %v", learner, err)
	}
	if len(learner.Course.Modules) != len(stored.Modules) || len(learner.Course.Assessments) != 1 || learner.Course.Assessments[0].Questions[0].Prompt != title || learner.Course.Assessments[0].Questions[0].Options[0].Text != title {
		t.Fatalf("imported learner structure or assessment translation = %#v", learner.Course)
	}
	assetBinding, err := courseRepo.GetPublishedAssetBindingByCourseAndVersionAndAssetKey(ctx, course.ID, stored.CourseVersion.Version, stored.AssetBindings[0].AssetKey)
	if err != nil || assetBinding.AssetKey != stored.AssetBindings[0].AssetKey || assetBinding.StorageObjectID != stored.AssetBindings[0].StorageObjectID {
		t.Fatalf("imported asset binding = %#v, %v", assetBinding, err)
	}
	var workspaceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM translations.course_translation WHERE source_course_version_id=$1`, stored.ID).Scan(&workspaceCount); err != nil {
		t.Fatal(err)
	}
	if workspaceCount != 0 {
		t.Fatalf("imported publication created %d workspaces", workspaceCount)
	}
	var importedOrigin string
	var workspaceID, creatorID *string
	var importID string
	if err := pool.QueryRow(ctx, `SELECT p.publication_origin,p.translation_id::text,w.creator_user_id::text,p.import_id::text FROM translations.translation_publication p LEFT JOIN translations.course_translation w ON w.id=p.translation_id WHERE p.id=$1`, publication.ID).Scan(&importedOrigin, &workspaceID, &creatorID, &importID); err != nil {
		t.Fatal(err)
	}
	if importedOrigin != string(translations.PublicationOriginImported) || workspaceID != nil || creatorID != nil || importID != record.ID.String() {
		t.Fatalf("imported database provenance origin=%q workspace=%v creator=%v import=%q", importedOrigin, workspaceID, creatorID, importID)
	}
	for _, statement := range []string{
		`UPDATE translations.translation_publication SET translation_id='10000000-0000-4000-8000-000000000001' WHERE id=$1`,
		`UPDATE translations.translation_publication SET import_id=NULL WHERE id=$1`,
		`UPDATE translations.translation_publication SET publication_origin='UNKNOWN' WHERE id=$1`,
		`UPDATE translations.translation_publication SET publication_origin='AUTHORING_PUBLICATION' WHERE id=$1`,
	} {
		if _, err := pool.Exec(ctx, statement, publication.ID); err == nil {
			t.Fatalf("invalid imported provenance accepted: %s", statement)
		}
	}

	rollbackCourse, err := courseRepo.CreateCourse(ctx, "imported-translation-rollback")
	if err != nil {
		t.Fatal(err)
	}
	rollback := input
	rollback.CourseVersion.CourseID = rollbackCourse.ID
	rollback.Publication.Imported = &courses.ImportedPublicationProvenance{ImportID: record.ID.String(), PackageFingerprint: record.PackageFingerprint, PortableSourceCourseID: record.PortableSourceCourseID, PortableSourceCourseVersionID: record.PortableSourceCourseVersionID, SourceVersion: rollback.CourseVersion.Version, ImportedAt: importedAt}
	// A second import record supplies truthful provenance for the rollback attempt.
	otherRecord, err := sqlc.New(pool).CreateImportRecord(ctx, sqlc.CreateImportRecordParams{PackageFingerprint: "9999999999999999999999999999999999999999999999999999999999999999", PortableSourceCourseID: "portable-rollback-course", PortableSourceCourseVersionID: "portable-rollback-v1", SourceVersion: "1.0.0", ImportedAt: pgtype.Timestamptz{Time: importedAt, Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	rollback.Publication.Imported.ImportID = otherRecord.ID.String()
	rollback.Publication.Imported.PackageFingerprint = otherRecord.PackageFingerprint
	rollback.Publication.Imported.PortableSourceCourseID = otherRecord.PortableSourceCourseID
	rollback.Publication.Imported.PortableSourceCourseVersionID = otherRecord.PortableSourceCourseVersionID
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rolledCourse, err := courseRepo.StoreImmutableCourseVersionInTx(ctx, tx, rollback)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	rolledTree, err := translations.SeedTranslationTree(rolledCourse)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	completeImportedTranslation(&rolledTree, title, description)
	rolledPublication, err := translationRepo.StoreImportedPublicationInTx(ctx, tx, rolledCourse, "es", rolledTree, translations.ImportedPublicationProvenance{ImportID: otherRecord.ID.String(), PackageFingerprint: otherRecord.PackageFingerprint, PortableSourceCourseID: otherRecord.PortableSourceCourseID, PortableSourceCourseVersionID: otherRecord.PortableSourceCourseVersionID, ImportedAt: importedAt}, importedAt)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO portability.import_record(package_fingerprint,portable_source_course_id,portable_source_course_version_id,source_version,imported_at) VALUES($1,'collision','collision','1.0.0',$2)`, otherRecord.PackageFingerprint, importedAt); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("expected downstream failure")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM courses.course_version WHERE id=$1`, rolledCourse.ID).Scan(&outside); err != nil || outside != 0 {
		t.Fatalf("rolled-back CourseVersion count=%d err=%v", outside, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM translations.translation_publication WHERE id=$1`, rolledPublication.ID).Scan(&outside); err != nil || outside != 0 {
		t.Fatalf("rolled-back publication count=%d err=%v", outside, err)
	}
	var nativePublicationID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM translations.translation_publication WHERE publication_origin='AUTHORING_PUBLICATION' LIMIT 1`).Scan(&nativePublicationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE translations.translation_publication SET import_id=$2 WHERE id=$1`, nativePublicationID, record.ID); err == nil {
		t.Fatal("authoring publication accepted import provenance")
	}
}

func completeImportedTranslation(tree *translations.TranslationTree, title, description string) {
	tree.Title, tree.Description = &title, &description
	for i := range tree.LearningObjectives {
		tree.LearningObjectives[i] = &title
	}
	for i := range tree.Modules {
		module := &tree.Modules[i]
		module.Title, module.Description = &title, &description
		for j := range module.Lessons {
			lesson := &module.Lessons[j]
			lesson.Title, lesson.Description = &title, &description
			for k := range lesson.LearningObjectives {
				lesson.LearningObjectives[k] = &title
			}
			for k := range lesson.ContentBlocks {
				if lesson.ContentBlocks[k].Type == courses.BlockImage {
					lesson.ContentBlocks[k].AltText = &title
					lesson.ContentBlocks[k].Caption = &description
				}
			}
		}
	}
	for i := range tree.Assessments {
		for j := range tree.Assessments[i].Questions {
			question := &tree.Assessments[i].Questions[j]
			question.Prompt = &title
			for k := range question.Options {
				question.Options[k].Text = &title
			}
		}
	}
}
