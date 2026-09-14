// archcheck enforces the first module import and SQL ownership boundaries.
package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const modulePrefix = "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/"
const platformPrefix = "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/platform"

var writeTarget = regexp.MustCompile(`(?i)\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM|MERGE\s+INTO)\s+([a-z_][a-z_0-9.]*)`)

func main() {
	contents, err := os.ReadFile("docs/table-owners.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var tableOwners map[string]string
	if err := json.Unmarshal(contents, &tableOwners); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := check("internal/modules", tableOwners); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(root string, tableOwners map[string]string) error {
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 2 {
			return nil
		}
		owner := parts[0]
		switch filepath.Ext(path) {
		case ".go":
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range file.Imports {
				literal, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if literal == platformPrefix || strings.HasPrefix(literal, platformPrefix+"/") {
					violations = append(violations, fmt.Sprintf("%s: %s cannot import the platform composition root", path, owner))
					continue
				}
				if !strings.HasPrefix(literal, modulePrefix) {
					continue
				}
				target := strings.Split(strings.TrimPrefix(literal, modulePrefix), "/")[0]
				if owner == "authoring" && target == "courses" && literal != modulePrefix+"courses" {
					violations = append(violations, fmt.Sprintf("%s: authoring may import Courses domain values only", path))
					continue
				}
				if owner == "authoring" && target == "identity" && literal != modulePrefix+"identity" {
					violations = append(violations, fmt.Sprintf("%s: authoring may import Identity domain actor only", path))
					continue
				}
				if forbidden(owner, target) {
					violations = append(violations, fmt.Sprintf("%s: %s cannot import %s", path, owner, target))
				}
			}
		case ".sql":
			if !strings.Contains(filepath.ToSlash(rel), "/db/query/") {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, match := range writeTarget.FindAllStringSubmatch(stripSQLComments(string(contents)), -1) {
				table := strings.ToLower(match[1])
				if tableOwners[table] != owner {
					violations = append(violations, fmt.Sprintf("%s: %s cannot write %s (owner: %s)", path, owner, table, tableOwners[table]))
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("architecture violations:\n%s", strings.Join(violations, "\n"))
	}
	return nil
}

func forbidden(owner, target string) bool {
	if owner == target {
		return false
	}
	if target == "administration" {
		return true
	}
	// Published Courses artifacts and read policy do not consult other domains.
	// References such as knowledge-check keys stay opaque within Courses.
	if owner == "courses" {
		return true
	}
	// Authoring may reuse immutable Courses value objects, never another module's
	// application or persistence adapters. The exact Courses path is checked above.
	if owner == "authoring" && target != "courses" && target != "identity" {
		return true
	}
	if owner != "administration" && (target == "notifications" || target == "search" || target == "audit") {
		return true
	}
	if owner != "infrastructure" && target == "infrastructure" {
		return true
	}
	return false
}

func stripSQLComments(sql string) string {
	var lines []string
	for _, line := range strings.Split(sql, "\n") {
		if before, _, ok := strings.Cut(line, "--"); ok {
			line = before
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
