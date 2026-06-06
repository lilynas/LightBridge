package modules

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ManifestFilename = "module.yaml"

func LoadManifestFile(path string) (*Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadManifestBytes(content)
}

func LoadManifestDir(dir string) (*Manifest, error) {
	return LoadManifestFile(filepath.Join(dir, ManifestFilename))
}

func LoadManifestBytes(content []byte) (*Manifest, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("parse module manifest: %w", err)
	}
	if !manifestHasTopLevelField(&root, "permissions") {
		return nil, fmt.Errorf("permissions is required")
	}

	var manifest Manifest
	if err := root.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parse module manifest: %w", err)
	}
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func manifestHasTopLevelField(root *yaml.Node, field string) bool {
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return false
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == field {
			return true
		}
	}
	return false
}
