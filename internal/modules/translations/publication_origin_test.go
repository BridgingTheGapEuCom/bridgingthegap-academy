package translations

import (
	"testing"
	"time"
)

func TestPublicationProvenanceBranches(t *testing.T) {
	authoring := PublicationProvenance{Origin: PublicationOriginAuthoring, Authoring: &AuthoringPublicationProvenance{TranslationID: "10000000-0000-4000-8000-000000000001", Revision: 2}}
	imported := PublicationProvenance{Origin: PublicationOriginImported, Imported: &ImportedPublicationProvenance{ImportID: "20000000-0000-4000-8000-000000000001", PackageFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PortableSourceCourseID: "portable-course", PortableSourceCourseVersionID: "portable-v1", ImportedAt: time.Now()}}
	if authoring.Validate() != nil || imported.Validate() != nil {
		t.Fatal("valid provenance rejected")
	}
	for _, invalid := range []PublicationProvenance{
		{Origin: PublicationOriginAuthoring, Authoring: authoring.Authoring, Imported: imported.Imported},
		{Origin: PublicationOriginImported, Authoring: authoring.Authoring, Imported: imported.Imported},
		{Origin: PublicationOriginImported},
		{Origin: "UNKNOWN", Imported: imported.Imported},
	} {
		if invalid.Validate() == nil {
			t.Fatalf("invalid provenance accepted: %#v", invalid)
		}
	}
}
