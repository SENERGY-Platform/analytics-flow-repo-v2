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

package api

import (
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util/slog_attr"
	gin_mw "github.com/SENERGY-Platform/gin-middleware"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// New godoc
// @title Analytics-Flow-Repo-V2 API
// @version 0.0.16
// @description For the administration of analytics flows.
// @license.name Apache-2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /
func New(srv Repo, staticHeader map[string]string, urlPrefix string) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	httpHandler := gin.New()
	httpHandler.RedirectTrailingSlash = false
	httpHandler.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS", "PUT"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	var middleware []gin.HandlerFunc
	middleware = append(
		middleware,
		gin_mw.StructLoggerHandler(
			util.Logger.With(attributes.LogRecordTypeKey, attributes.HttpAccessLogRecordTypeVal),
			attributes.Provider,
			[]string{HealthCheckPath},
			nil,
			requestIDGenerator,
		),
	)
	middleware = append(middleware,
		gin_mw.StaticHeaderHandler(staticHeader),
		requestid.New(requestid.WithCustomHeaderStrKey(HeaderRequestID)),
		gin_mw.ErrorHandler(GetStatusCode, ", "),
		gin_mw.StructRecoveryHandler(util.Logger, gin_mw.DefaultRecoveryFunc),
	)
	httpHandler.Use(middleware...)
	httpHandler.UseRawPath = true
	httpHandlerWithPrefix := httpHandler.Group(urlPrefix)
	setRoutes, err := routes.Set(srv, httpHandlerWithPrefix)
	if err != nil {
		return nil, err
	}
	for _, route := range setRoutes {
		util.Logger.Debug("http route", attributes.MethodKey, route[0], attributes.PathKey, route[1])
	}
	return httpHandler, nil
}

func requestIDGenerator(gc *gin.Context) (string, any) {
	return slog_attr.RequestIDKey, requestid.Get(gc)
}
