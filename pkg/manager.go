package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Package represents a SQU1D++ package
type Package struct {
	Name        string
	Path        string
	Version     string
	Description string
	Files       []string
}

// Manager handles package operations
type Manager struct {
	packageDir string
	packages   map[string]*Package
}

// NewManager creates a new package manager
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	packageDir := filepath.Join(homeDir, ".cache", "squ1dlang")
	
	// Create package directory if it doesn't exist
	os.MkdirAll(packageDir, 0755)
	
	return &Manager{
		packageDir: packageDir,
		packages:   make(map[string]*Package),
	}
}

// CreatePackage creates a new package structure
func (pm *Manager) CreatePackage(name, description string) error {
	packagePath := filepath.Join(pm.packageDir, name)
	
	// Check if package already exists
	if _, err := os.Stat(packagePath); err == nil {
		return fmt.Errorf("package '%s' already exists", name)
	}
	
	// Create package directory
	err := os.MkdirAll(packagePath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create package directory: %v", err)
	}
	
	// Create __init__.sqd file
	initFile := filepath.Join(packagePath, "__init__.sqd")
	initContent := fmt.Sprintf(`# Package: %s
# Description: %s
# Version: 1.0.0

# Package initialization code goes here
`, name, description)
	
	err = os.WriteFile(initFile, []byte(initContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to create __init__.sqd: %v", err)
	}
	
	// Create package.json metadata file
	metadataFile := filepath.Join(packagePath, "package.json")
	metadataContent := fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "description": "%s",
  "main": "__init__.sqd",
  "files": ["__init__.sqd"]
}`, name, description)
	
	err = os.WriteFile(metadataFile, []byte(metadataContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to create package.json: %v", err)
	}
	
	// Create README.md
	readmeFile := filepath.Join(packagePath, "README.md")
	readmeContent := fmt.Sprintf(`# %s

%s

## Installation

This package is installed in your local SQU1D++ package directory.

## Usage

%s
`, name, description, "include(\""+name+"\")")
	
	err = os.WriteFile(readmeFile, []byte(readmeContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to create README.md: %v", err)
	}
	
	fmt.Printf("Package '%s' created successfully at %s\n", name, packagePath)
	return nil
}

// ListPackages lists all installed packages
func (pm *Manager) ListPackages() ([]*Package, error) {
	var packages []*Package
	
	entries, err := os.ReadDir(pm.packageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read package directory: %v", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			packagePath := filepath.Join(pm.packageDir, entry.Name())
			pkg, err := pm.loadPackage(entry.Name(), packagePath)
			if err != nil {
				fmt.Printf("Warning: failed to load package '%s': %v\n", entry.Name(), err)
				continue
			}
			packages = append(packages, pkg)
		}
	}
	
	return packages, nil
}

// loadPackage loads package information from disk
func (pm *Manager) loadPackage(name, path string) (*Package, error) {
	pkg := &Package{
		Name: name,
		Path: path,
	}
	
	// Try to read package.json
	metadataFile := filepath.Join(path, "package.json")
	if data, err := os.ReadFile(metadataFile); err == nil {
		// Simple JSON parsing (in a real implementation, you'd use a proper JSON parser)
		content := string(data)
		if strings.Contains(content, `"version"`) {
			// Extract version (simplified)
			start := strings.Index(content, `"version": "`) + 11
			end := strings.Index(content[start:], `"`)
			if start > 10 && end > 0 {
				pkg.Version = content[start : start+end]
			}
		}
		if strings.Contains(content, `"description"`) {
			// Extract description (simplified)
			start := strings.Index(content, `"description": "`) + 16
			end := strings.Index(content[start:], `"`)
			if start > 15 && end > 0 {
				pkg.Description = content[start : start+end]
			}
		}
	}
	
	// List files in package
	files, err := filepath.Glob(filepath.Join(path, "*.sqd"))
	if err == nil {
		for _, file := range files {
			pkg.Files = append(pkg.Files, filepath.Base(file))
		}
	}
	
	return pkg, nil
}

// RemovePackage removes a package
func (pm *Manager) RemovePackage(name string) error {
	packagePath := filepath.Join(pm.packageDir, name)
	
	// Check if package exists
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package '%s' does not exist", name)
	}
	
	// Remove package directory
	err := os.RemoveAll(packagePath)
	if err != nil {
		return fmt.Errorf("failed to remove package: %v", err)
	}
	
	fmt.Printf("Package '%s' removed successfully\n", name)
	return nil
}

// GetPackagePath returns the path to a package
func (pm *Manager) GetPackagePath(name string) (string, error) {
	packagePath := filepath.Join(pm.packageDir, name)
	
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("package '%s' not found", name)
	}
	
	return packagePath, nil
}

// GlobalManager is the global package manager instance
var GlobalManager = NewManager()
