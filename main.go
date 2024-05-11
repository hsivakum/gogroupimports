package gogroupimports

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	SelfModule             string   `json:"selfModule"`
	InternalPrivateDomains []string `json:"internalPrivateDomains"`
}

func Run(filename string, metaData map[string]interface{}) ([]byte, error) {
	marshal, err := json.Marshal(metaData)
	if err != nil {
		return nil, err
	}
	var settings Settings
	err = json.Unmarshal(marshal, &settings)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()

	// Parse the source file
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("Failed to parse file: %v", err)
	}

	importGroups, err := getImportGroups(fset, node, settings)
	if err != nil {
		log.Fatalf("Error getting import groups: %v", err)
	}

	// Check if imports are properly grouped and have line breaks between groups
	if !areImportsGrouped(importGroups) {
		return nil, fmt.Errorf("Warning: Imports are not properly grouped in file %s\n.", filename)
	}

	// Check for line breaks between import groups
	for i, group := range importGroups {
		if i > 0 && group.startLine != importGroups[i-1].endLine+2 {
			return nil, fmt.Errorf("Warning: Missing single line break before %d\n", group.startLine)
		}
	}

	return nil, err
}

// ImportGroup represents a group of consecutive import declarations
type ImportGroup struct {
	startLine  int    // Start line of the group
	endLine    int    // End line of the group
	importType string // Type of import: "builtin", "public_open_source", "internal_private_or_own_module"
}

// getImportGroups extracts import groups from the AST
func getImportGroups(fset *token.FileSet, node *ast.File, settings Settings) ([]ImportGroup, error) {
	var groups []ImportGroup
	var currentGroup *ImportGroup

	for _, decl := range node.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			for _, spec := range genDecl.Specs {
				importSpec := spec.(*ast.ImportSpec)
				importPath := importSpec.Path.Value[1 : len(importSpec.Path.Value)-1] // Remove quotes

				// Determine the type of import and group accordingly
				importType := getImportType(importPath, settings)

				// Start a new group if necessary
				if currentGroup == nil || currentGroup.importType != importType {
					if currentGroup != nil {
						groups = append(groups, *currentGroup)
					}
					currentGroup = &ImportGroup{
						startLine:  fset.Position(importSpec.Pos()).Line,
						endLine:    fset.Position(importSpec.End()).Line,
						importType: importType,
					}
				} else {
					// Update the end line of the current group
					currentGroup.endLine = fset.Position(importSpec.End()).Line
				}
			}
		}
	}

	if currentGroup != nil {
		groups = append(groups, *currentGroup)
	}

	return groups, nil
}

// getImportType determines the type of import
func getImportType(path string, settings Settings) string {
	if isInternalPrivateImport(path, settings) {
		return "internal_private"
	} else if strings.HasPrefix(path, settings.SelfModule) {
		return "own_module"
	} else if isBuiltinImport(path) {
		return "builtin"
	} else {
		return "public_open_source_or_third_party"
	}
}

// areImportsGrouped checks if imports are properly grouped
func areImportsGrouped(groups []ImportGroup) bool {
	// Define the correct sequence of import types
	expectedSequence := []string{"builtin", "public_open_source_or_third_party", "internal_private", "own_module"}

	// Check if the actual import sequence matches the expected sequence
	for i, group := range groups {
		if i < len(expectedSequence) && group.importType != expectedSequence[i] {
			return false
		}
	}
	return true
}

// Helper functions to check import types

func isInternalPrivateImport(path string, settings Settings) bool {
	for _, domain := range settings.InternalPrivateDomains {
		if strings.Contains(path, domain) {
			return true
		}
	}
	return false
}

func isBuiltinImport(path string) bool {
	// Check if the import path belongs to a built-in package
	_, err := os.Stat(filepath.Join(build.Default.GOROOT, "src", path))
	return err == nil
}
