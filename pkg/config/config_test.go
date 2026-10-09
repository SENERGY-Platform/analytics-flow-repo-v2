/*
 * Copyright 2026 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sb_util "github.com/SENERGY-Platform/go-service-base/util"
)

var mongoEnvVars = []string{"MONGO_URL", "MONGO_USER", "MONGO_PASSWORD", "MONGO_AUTH_SOURCE", "MONGO_DATABASE"}

// unsetMongoEnv removes the variables for the test; t.Setenv restores them afterwards.
func unsetMongoEnv(t *testing.T) {
	for _, k := range mongoEnvVars {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewMongoDefaults(t *testing.T) {
	unsetMongoEnv(t)
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MongoUrl != "mongodb://localhost:27017" {
		t.Errorf("MongoUrl = %q", cfg.MongoUrl)
	}
	if cfg.MongoUser != "" {
		t.Errorf("MongoUser = %q", cfg.MongoUser)
	}
	if cfg.MongoPassword.Value() != "" {
		t.Error("MongoPassword is not empty by default")
	}
	if cfg.MongoAuthSource != "admin" {
		t.Errorf("MongoAuthSource = %q", cfg.MongoAuthSource)
	}
	if cfg.MongoDatabase != "analytics_flow_repo" {
		t.Errorf("MongoDatabase = %q", cfg.MongoDatabase)
	}
}

func TestNewMongoFromEnv(t *testing.T) {
	unsetMongoEnv(t)
	t.Setenv("MONGO_URL", "mongodb://mongo-0.mongo:27017/?replicaSet=rs0")
	t.Setenv("MONGO_USER", "analytics-flow-repo")
	t.Setenv("MONGO_PASSWORD", "s3cret")
	t.Setenv("MONGO_AUTH_SOURCE", "users")
	t.Setenv("MONGO_DATABASE", "flows_db")
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MongoUrl != "mongodb://mongo-0.mongo:27017/?replicaSet=rs0" {
		t.Errorf("MongoUrl = %q", cfg.MongoUrl)
	}
	if cfg.MongoUser != "analytics-flow-repo" {
		t.Errorf("MongoUser = %q", cfg.MongoUser)
	}
	if cfg.MongoPassword.Value() != "s3cret" {
		t.Error("MongoPassword not taken from env")
	}
	if cfg.MongoAuthSource != "users" {
		t.Errorf("MongoAuthSource = %q", cfg.MongoAuthSource)
	}
	if cfg.MongoDatabase != "flows_db" {
		t.Errorf("MongoDatabase = %q", cfg.MongoDatabase)
	}
}

func TestNewSmartServiceRepositoryUrl(t *testing.T) {
	tests := []struct {
		name string
		file string
		env  string
		want string
	}{
		{"default", "", "", "http://api.smart-service-repository:8080"},
		{"default when the file omits it", `{}`, "", "http://api.smart-service-repository:8080"},
		{"file", `{"smart_service_repository_url": "http://file:1"}`, "", "http://file:1"},
		{"environment wins over the file", `{"smart_service_repository_url": "http://file:1"}`, "http://env:2", "http://env:2"},
		// The repo refuses an empty address at startup, see TestNewRefusesAnEmptySmartServiceRepositoryUrl.
		{"empty in the file stays empty", `{"smart_service_repository_url": ""}`, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unsetMongoEnv(t)
			// An empty variable counts as set, so unset it for the cases without one.
			t.Setenv("SMART_SERVICE_REPOSITORY_URL", tc.env)
			if tc.env == "" {
				if err := os.Unsetenv("SMART_SERVICE_REPOSITORY_URL"); err != nil {
					t.Fatal(err)
				}
			}
			path := ""
			if tc.file != "" {
				path = filepath.Join(t.TempDir(), "config.json")
				if err := os.WriteFile(path, []byte(tc.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := New(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.SmartServiceRepositoryUrl != tc.want {
				t.Errorf("SmartServiceRepositoryUrl = %q, want %q", cfg.SmartServiceRepositoryUrl, tc.want)
			}
		})
	}
}

func TestConfigOutputHidesMongoPassword(t *testing.T) {
	const password = "pw-must-not-appear-7f3a"
	unsetMongoEnv(t)
	t.Setenv("MONGO_PASSWORD", password)
	cfg, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"startup log (ToJsonStr)": sb_util.ToJsonStr(cfg),
		"%v":                      fmt.Sprintf("%v", cfg),
		"%+v":                     fmt.Sprintf("%+v", cfg),
	}
	for name, out := range outputs {
		if strings.Contains(out, password) {
			t.Errorf("%s contains the password", name)
		}
	}
}
