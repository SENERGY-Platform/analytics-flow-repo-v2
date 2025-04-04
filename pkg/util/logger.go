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
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/config"
	sb_logger "github.com/SENERGY-Platform/go-service-base/logger"
	"os"
)

var Logger *sb_logger.Logger

func InitLogger(c config.LoggerConfig) (out *os.File, err error) {
	Logger, out, err = sb_logger.New(c.Level, c.Path, c.FileName, c.Prefix, c.Utc, c.Terminal, c.Microseconds)
	Logger.SetLevelPrefix("ERROR ", "WARNING ", "INFO ", "DEBUG ")
	return
}
