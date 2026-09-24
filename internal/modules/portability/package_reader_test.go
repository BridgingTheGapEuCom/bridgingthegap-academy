package portability

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
)

func TestReaderValidatesExportAndBuildsSafePreview(t *testing.T) {
	source := packageSource(t, "1.0.0")
	storage := packageStorage{objects: map[assets.StorageObjectID][]byte{assets.StorageObjectID(source.AssetBindings[0].StorageObjectID): []byte("image bytes")}}
	service, _ := translations.NewService(packageSourceRepo{source}, packageTranslationRepo{}, time.Now)
	exporter, _ := NewExporter(packageSourceRepo{source}, service, storage, func() time.Time { return time.Unix(1, 0) })
	var archive bytes.Buffer
	if err := exporter.Write(context.Background(), source.CourseVersion.CourseID, source.CourseVersion.Version, &archive); err != nil {
		t.Fatal(err)
	}
	validated, err := NewReader(DefaultLimits()).Read(context.Background(), bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	preview := validated.Preview()
	if preview.Title != "Portable course" || preview.Version != "1.0.0" || preview.AssetCount != 1 || preview.AssessmentCount != 1 || preview.LessonCount != 1 || preview.FormatVersion != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	encoded, _ := json.Marshal(preview)
	if bytes.Contains(encoded, []byte("option-b")) || bytes.Contains(encoded, []byte("StorageObjectID")) {
		t.Fatalf("preview leaked protected data: %s", encoded)
	}
}

func TestReaderRejectsUnsafeAndDuplicateEntries(t *testing.T) {
	for name, code := range map[string]ErrorCode{"../manifest.json": ErrUnsafeArchivePath, "/manifest.json": ErrUnsafeArchivePath, `..\\manifest.json`: ErrUnsafeArchivePath} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			w, _ := z.Create(name)
			_, _ = w.Write([]byte("{}"))
			_ = z.Close()
			_, err := NewReader(DefaultLimits()).Read(context.Background(), bytes.NewReader(b.Bytes()))
			if !IsPackageError(err, code) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for range 2 {
		w, _ := z.Create(manifestPath)
		_, _ = w.Write([]byte("{}"))
	}
	_ = z.Close()
	_, err := NewReader(DefaultLimits()).Read(context.Background(), bytes.NewReader(b.Bytes()))
	if !IsPackageError(err, ErrInvalidArchive) {
		t.Fatalf("duplicate error=%v", err)
	}
}

func TestReaderRejectsChecksumTamperingAndLimits(t *testing.T) {
	source := packageSource(t, "1.0.0")
	storage := packageStorage{objects: map[assets.StorageObjectID][]byte{assets.StorageObjectID(source.AssetBindings[0].StorageObjectID): []byte("image bytes")}}
	service, _ := translations.NewService(packageSourceRepo{source}, packageTranslationRepo{}, time.Now)
	exporter, _ := NewExporter(packageSourceRepo{source}, service, storage, time.Now)
	var archive bytes.Buffer
	if err := exporter.Write(context.Background(), source.CourseVersion.CourseID, source.CourseVersion.Version, &archive); err != nil {
		t.Fatal(err)
	}
	files := unzip(t, archive.Bytes())
	files[coursePath] = []byte(`{"title":"tampered"}`)
	tampered := zipFiles(t, files)
	_, err := NewReader(DefaultLimits()).Read(context.Background(), bytes.NewReader(tampered))
	if !IsPackageError(err, ErrChecksumMismatch) {
		t.Fatalf("checksum error=%v", err)
	}
	_, err = NewReader(Limits{MaxCompressedBytes: 1}).Read(context.Background(), bytes.NewReader(archive.Bytes()))
	if !IsPackageError(err, ErrPackageTooLarge) {
		t.Fatalf("limit error=%v", err)
	}
}

func zipFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, body := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func FuzzSafeArchivePath(f *testing.F) {
	for _, s := range []string{"manifest.json", "../x", "/x", `..\\x`, "assets/a/content"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { _, _ = safeArchivePath(s) })
}
func FuzzStrictJSON(f *testing.F) {
	f.Add([]byte(`{"format":"bridging-the-gap-course"}`))
	f.Fuzz(func(t *testing.T, b []byte) { var m Manifest; _ = strictDecode(b, &m) })
}
