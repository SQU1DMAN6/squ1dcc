package pkgmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Package struct {
	Name        string
	Path        string
	Version     string
	Description string
	Files       []string
}

type Manager struct {
	packageDir string
	packages   map[string]*Package
}

func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	packageDir := filepath.Join(homeDir, ".squ1dlang", "packages")

	os.MkdirAll(packageDir, 0755)

	return &Manager{
		packageDir: packageDir,
		packages:   make(map[string]*Package),
	}
}

func (pm *Manager) CreatePackage(name, description string) error {
	packagePath := filepath.Join(pm.packageDir, name)

	if _, err := os.Stat(packagePath); err == nil {
		return fmt.Errorf("Package '%s' already exists", name)
	}

	err := os.MkdirAll(packagePath, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create package directory: %v", err)
	}

	initFile := filepath.Join(packagePath, "__init__.sqd")
	initContent := fmt.Sprintf(`# Package: %s
# Description: %s
# Version: 1.0.0

# Package initialization code goes here
`, name, description)

	err = os.WriteFile(initFile, []byte(initContent), 0644)
	if err != nil {
		return fmt.Errorf("Failed to create __init__.sqd: %v", err)
	}

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
		return fmt.Errorf("Failed to create package.json: %v", err)
	}

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
		return fmt.Errorf("Failed to create README.md: %v", err)
	}

	fmt.Printf("Package '%s' created successfully at %s\n", name, packagePath)
	return nil
}

func (pm *Manager) ListPackages() ([]*Package, error) {
	var packages []*Package

	entries, err := os.ReadDir(pm.packageDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to read package directory: %v", err)
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

func (pm *Manager) loadPackage(name, path string) (*Package, error) {
	pkg := &Package{
		Name: name,
		Path: path,
	}

	metadataFile := filepath.Join(path, "package.json")
	if data, err := os.ReadFile(metadataFile); err == nil {
		content := string(data)
		if strings.Contains(content, `"version"`) {
			start := strings.Index(content, `"version": "`) + 11
			end := strings.Index(content[start:], `"`)
			if start > 10 && end > 0 {
				pkg.Version = content[start : start+end]
			}
		}
		if strings.Contains(content, `"description"`) {
			start := strings.Index(content, `"description": "`) + 16
			end := strings.Index(content[start:], `"`)
			if start > 15 && end > 0 {
				pkg.Description = content[start : start+end]
			}
		}
	}

	files, err := filepath.Glob(filepath.Join(path, "*.sqd"))
	if err == nil {
		for _, file := range files {
			pkg.Files = append(pkg.Files, filepath.Base(file))
		}
	}

	return pkg, nil
}

func (pm *Manager) RemovePackage(name string) error {
	packagePath := filepath.Join(pm.packageDir, name)

	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("Package '%s' does not exist", name)
	}

	err := os.RemoveAll(packagePath)
	if err != nil {
		return fmt.Errorf("Failed to remove package: %v", err)
	}

	fmt.Printf("Package '%s' removed successfully\n", name)
	return nil
}

func (pm *Manager) GetPackagePath(name string) (string, error) {
	packagePath := filepath.Join(pm.packageDir, name)

	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("Package '%s' not found", name)
	}

	return packagePath, nil
}

var GlobalManager = NewManager()
