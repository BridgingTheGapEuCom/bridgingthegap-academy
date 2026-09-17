package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForbiddenImports(t *testing.T) {
	tests := []struct {
		owner, target string
		want          bool
	}{
		{"courses", "administration", true},
		{"courses", "authoring", true},
		{"courses", "publishing", true},
		{"courses", "identity", true},
		{"courses", "assessments", true},
		{"courses", "learning", true},
		{"learning", "notifications", true},
		{"courses", "infrastructure", true},
		{"administration", "courses", false},
		{"publishing", "courses", false},
		{"courses", "courses", false},
		{"authoring", "identity", false},
		{"authoring", "courses", false},
		{"authoring", "audit", true},
	}
	for _, tt := range tests {
		if got := forbidden(tt.owner, tt.target); got != tt.want {
			t.Errorf("forbidden(%q, %q) = %v, want %v", tt.owner, tt.target, got, tt.want)
		}
	}
}

func TestAuthoringMayUseDomainActorButNotIdentityPersistence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "authoring", "actor.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package authoring\nimport _ \""+modulePrefix+"identity\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := check(root, nil); err != nil {
		t.Fatalf("trusted actor domain import rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte("package authoring\nimport _ \""+modulePrefix+"identity/postgres\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := check(root, nil); err == nil || !strings.Contains(err.Error(), "Identity domain actor only") {
		t.Fatalf("Identity persistence import accepted: %v", err)
	}
	if err := os.WriteFile(path, []byte("package authoring\nimport _ \""+modulePrefix+"courses/postgres\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := check(root, nil); err == nil || !strings.Contains(err.Error(), "Courses domain values only") {
		t.Fatalf("Courses persistence import accepted: %v", err)
	}
}

func TestAssessmentsRemainNeutralAndAuthoringMayUseItsDomainValues(t *testing.T) {
	if forbidden("assessments", "authoring") == false {
		t.Fatal("Assessments must not import Authoring")
	}
	if forbidden("authoring", "assessments") {
		t.Fatal("Authoring should be able to use neutral Assessment domain values")
	}
	root := t.TempDir()
	path := filepath.Join(root, "authoring", "assessment.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package authoring\nimport _ \""+modulePrefix+"assessments\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := check(root, nil); err != nil {
		t.Fatalf("Assessment domain import rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte("package authoring\nimport _ \""+modulePrefix+"assessments/postgres\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := check(root, nil); err == nil || !strings.Contains(err.Error(), "Assessments domain values only") {
		t.Fatalf("Assessment persistence import accepted: %v", err)
	}
}

func TestCheckRejectsForbiddenImportAndCrossModuleWrite(t *testing.T) {
	root := t.TempDir()
	goPath := filepath.Join(root, "courses", "bad.go")
	if err := os.MkdirAll(filepath.Dir(goPath), 0755); err != nil {
		t.Fatal(err)
	}
	goFile := "package courses\nimport _ \"" + modulePrefix + "authoring\"\n"
	if err := os.WriteFile(goPath, []byte(goFile), 0644); err != nil {
		t.Fatal(err)
	}
	queryPath := filepath.Join(root, "courses", "db", "query", "bad.sql")
	if err := os.MkdirAll(filepath.Dir(queryPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queryPath, []byte("UPDATE authoring_drafts SET state = 'x';"), 0644); err != nil {
		t.Fatal(err)
	}
	err := check(root, map[string]string{"authoring_drafts": "authoring"})
	if err == nil || !strings.Contains(err.Error(), "cannot import authoring") || !strings.Contains(err.Error(), "cannot write authoring_drafts") {
		t.Fatalf("expected both boundary violations, got %v", err)
	}
}
