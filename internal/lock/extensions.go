// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package lock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/RhythmShahriar/rhythm/kit/config"
)

// ExtensionsSchema is the version of the extensions file format.
const ExtensionsSchema = "rhythm-extensions/v1"

// ExtensionsFile is the extensions file's name in the config folder.
const ExtensionsFile = "extensions.json"

// configDirEnv moves the config folder, as kit/config will once its
// RHYTHM_CONFIG_DIR support is released; until then the store reads it
// itself so tests and CI can keep the store out of the real home folder.
const configDirEnv = "RHYTHM_CONFIG_DIR"

// Extension is a third-party tool the user added: `rhythm <Name>` runs
// rhythm-<Name> from Repo's releases.
type Extension struct {
	Name   string `json:"name"`             // "zap"
	Repo   string `json:"repo"`             // GitHub owner/name
	Signer string `json:"signer,omitempty"` // who must have signed its releases; empty means unchecked
}

// Extensions is the list of installed extensions, kept in
// extensions.json in the Rhythm config folder. Each change rewrites the
// file atomically.
type Extensions struct {
	path string
}

// OpenExtensions returns the store in the Rhythm config folder
// (RHYTHM_CONFIG_DIR when set, else ~/.config/rhythm or the OS
// equivalent). Nothing is created until the first Add.
func OpenExtensions() (*Extensions, error) {
	dir := os.Getenv(configDirEnv)
	if dir != "" && !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("%s must be an absolute path, not %q", configDirEnv, dir)
	}
	if dir == "" {
		var err error
		if dir, err = config.Dir(); err != nil {
			return nil, err
		}
	}
	return ExtensionsAt(filepath.Join(dir, ExtensionsFile)), nil
}

// ExtensionsAt returns the store kept in the file at path.
func ExtensionsAt(path string) *Extensions { return &Extensions{path: path} }

// Path is the file the store is kept in.
func (s *Extensions) Path() string { return s.path }

type extensionsDoc struct {
	Schema     string      `json:"schema"`
	Extensions []Extension `json:"extensions"`
}

// List returns the installed extensions sorted by name; none when the
// file doesn't exist yet.
func (s *Extensions) List() ([]Extension, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc extensionsDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	if doc.Schema != ExtensionsSchema {
		return nil, fmt.Errorf("%s: schema %q isn't %q", s.path, doc.Schema, ExtensionsSchema)
	}
	for _, x := range doc.Extensions {
		if err := x.validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", s.path, err)
		}
	}
	slices.SortFunc(doc.Extensions, compareExtensions)
	return doc.Extensions, nil
}

// Add records x, replacing an extension of the same name (a reinstall
// from another repo or signer).
func (s *Extensions) Add(x Extension) error {
	if err := x.validate(); err != nil {
		return err
	}
	list, err := s.List()
	if err != nil {
		return err
	}
	list = slices.DeleteFunc(list, func(e Extension) bool { return e.Name == x.Name })
	return s.save(append(list, x))
}

// Remove drops the named extension and reports whether it was there.
func (s *Extensions) Remove(name string) (bool, error) {
	list, err := s.List()
	if err != nil {
		return false, err
	}
	n := len(list)
	list = slices.DeleteFunc(list, func(e Extension) bool { return e.Name == name })
	if len(list) == n {
		return false, nil
	}
	return true, s.save(list)
}

func (s *Extensions) save(list []Extension) error {
	slices.SortFunc(list, compareExtensions)
	if list == nil {
		list = []Extension{}
	}
	data, err := marshal(extensionsDoc{ExtensionsSchema, list})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return writeAtomic(s.path, data, 0o600)
}

func compareExtensions(a, b Extension) int { return strings.Compare(a.Name, b.Name) }

func (x Extension) validate() error {
	switch {
	case !nameRE.MatchString(x.Name):
		return fmt.Errorf("extension name %q isn't a tool name", x.Name)
	case !repoRE.MatchString(x.Repo):
		return fmt.Errorf("extension %q: repo %q isn't owner/name", x.Name, x.Repo)
	}
	return nil
}
