/*
 * Copyright 2025 InfAI (CC SES)
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

import "time"
import "github.com/y-du/go-log-level/level"
import "github.com/SENERGY-Platform/go-service-base/config-hdl"

type Config struct {
	ServerPort       int           `json:"server_port" env_var:"SERVER_PORT"`
	Logger           LoggerConfig  `json:"logger" env_var:"LOGGER_CONFIG"`
	MongoUrl         string        `json:"mongo_url" env_var:"MONGO_URL"`
	HttpTimeout      time.Duration `json:"http_timeout" env_var:"HTTP_TIMEOUT"`
	PermissionsV2Url string        `json:"permissions_v2_url" env_var:"PERMISSIONS_V2_URL"`
	OperatorRepoUrl  string        `json:"operator_repo_url" env_var:"OPERATOR_REPO_URL"`
}

type LoggerConfig struct {
	Level        level.Level `json:"level" env_var:"LOGGER_LEVEL"`
	Utc          bool        `json:"utc" env_var:"LOGGER_UTC"`
	Path         string      `json:"path" env_var:"LOGGER_PATH"`
	FileName     string      `json:"file_name" env_var:"LOGGER_FILE_NAME"`
	Terminal     bool        `json:"terminal" env_var:"LOGGER_TERMINAL"`
	Microseconds bool        `json:"microseconds" env_var:"LOGGER_MICROSECONDS"`
	Prefix       string      `json:"prefix" env_var:"LOGGER_PREFIX"`
}

func New(path string) (*Config, error) {
	cfg := Config{
		ServerPort: 8080,
		Logger: LoggerConfig{
			Level:        level.Warning,
			Utc:          true,
			Microseconds: true,
			Terminal:     true,
		},
		MongoUrl:         "localhost:27017",
		HttpTimeout:      time.Second * 30,
		PermissionsV2Url: "http://permv2.permissions:8080",
		OperatorRepoUrl:  "http://operator-repo:8080",
	}
	err := config_hdl.Load(&cfg, nil, envTypeParser, nil, path)
	return &cfg, err
}
