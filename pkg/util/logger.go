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

package util

import (
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/config"
	sb_logger "github.com/SENERGY-Platform/go-service-base/logger"
	structlogger "github.com/SENERGY-Platform/go-service-base/struct-logger"
)

var Logger *sb_logger.Logger

var StructLogger *slog.Logger

func InitLogger(c config.LoggerConfig) (out *os.File, err error) {
	Logger, out, err = sb_logger.New(c.Level, c.Path, c.FileName, c.Prefix, c.Utc, c.Terminal, c.Microseconds)
	Logger.SetLevelPrefix("ERROR ", "WARNING ", "INFO ", "DEBUG ")
	return
}

func parseLevel(level sb_logger.Level) string {
	var levelStr = [5]string{
		"off",
		"error",
		"warning",
		"info",
		"debug",
	}
	return levelStr[level]
}

func InitStructLogger(c config.LoggerConfig) *slog.Logger {
	if StructLogger == nil {
		info, ok := debug.ReadBuildInfo()
		project := ""
		org := ""
		if ok {
			if parts := strings.Split(info.Main.Path, "/"); len(parts) > 2 {
				project = strings.Join(parts[2:], "/")
				org = strings.Join(parts[:2], "/")
			}
		}
		StructLogger = structlogger.New(
			structlogger.Config{
				Handler:    structlogger.JsonHandlerSelector,
				Level:      parseLevel(c.Level),
				TimeFormat: time.RFC3339Nano,
				TimeUtc:    true,
				AddMeta:    true,
			},
			os.Stdout,
			org,
			project,
		)
	}
	return StructLogger
}
