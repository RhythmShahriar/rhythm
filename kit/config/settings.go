// Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RhythmShahriar/rhythm/kit/cache"
)

// SettingsFile is the file in the config directory that holds the
// preferences `rhythm config` sets, as one JSON object of strings.
const SettingsFile = "settings.json"

// Setting returns the value saved for key, or "" when none is. A missing
// settings file is no error; one that can't be read or parsed is, so a
// caller can say so rather than act on a default the user changed.
func Setting(key string) (string, error) {
	s, _, err := readSettings()
	if err != nil {
		return "", err
	}
	return s[key], nil
}

// SetSetting saves value for key, or removes key when value is "". The
// file is rewritten atomically, so tools reading it at the same moment
// see the old settings or the new ones.
func SetSetting(key, value string) error {
	s, path, err := readSettings()
	if err != nil {
		return err
	}
	if value == "" {
		delete(s, key)
	} else {
		s[key] = value
	}
	return cache.WriteJSON(path, s)
}

func readSettings() (map[string]string, string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, SettingsFile)
	s := map[string]string{}
	if err := cache.ReadJSON(path, &s); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, path, nil
		}
		return nil, path, fmt.Errorf("reading %s: %w", path, err)
	}
	if s == nil { // the file held null
		s = map[string]string{}
	}
	return s, path, nil
}
