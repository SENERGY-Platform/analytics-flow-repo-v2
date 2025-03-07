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
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/models"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"slices"
	"strings"
)

// getInfoH godoc
// @Summary Get service info
// @Description	Get basic service and runtime information.
// @Tags Info
// @Produce	json
// @Success	200 {object} lib.SrvInfo "info"
// @Failure	500 {string} string "error message"
// @Router /info [get]
func getInfoH(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/info", func(gc *gin.Context) {
		gc.JSON(http.StatusOK, srv.SrvInfo(gc.Request.Context()))
	}
}

func putFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodPut, "/flow/", func(gc *gin.Context) {
		var request models.Flow
		if err := gc.ShouldBindJSON(&request); err != nil {
			_ = gc.Error(err)
			return
		}
		err := srv.CreateFlow(request, getUserId(gc))
		if err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Status(http.StatusCreated)
	}
}

func postFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodPost, "/flow/:id/", func(gc *gin.Context) {
		var request models.Flow
		if err := gc.ShouldBindJSON(&request); err != nil {
			_ = gc.Error(err)
			return
		}
		err := srv.UpdateFlow(gc.Param("id"), request, getUserId(gc))
		if err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Status(http.StatusOK)
	}
}

func deleteFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, "/flow/:id/", func(gc *gin.Context) {
		err := srv.DeleteFlow(gc.Param("id"), getUserId(gc))
		if err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Status(http.StatusNoContent)
	}
}

func getAll(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/flow", func(gc *gin.Context) {
		args := gc.Request.URL.Query()
		flows, err := srv.GetFlows(getUserId(gc), args)
		if err != nil {
			_ = gc.Error(err)
			return
		}
		gc.JSON(http.StatusOK, flows)
	}
}

func getFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/flow/:id", func(gc *gin.Context) {
		flow, err := srv.GetFlow(gc.Param("id"), getUserId(gc))
		if err != nil {
			_ = gc.Error(err)
			return
		}
		gc.JSON(http.StatusOK, flow)
	}
}

func getHealthCheckH(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, HealthCheckPath, func(gc *gin.Context) {
		err := srv.HealthCheck(gc.Request.Context())
		if err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Status(http.StatusOK)
	}
}

func getSwaggerDocH(_ Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/doc", func(gc *gin.Context) {
		if _, err := os.Stat("docs/swagger.json"); err != nil {
			_ = gc.Error(err)
			return
		}
		gc.Header("Content-Type", gin.MIMEJSON)
		gc.File("docs/swagger.json")
	}
}

func getUserId(c *gin.Context) (userId string) {
	forUser := c.Query("for_user")
	if forUser != "" {
		roles := strings.Split(c.GetHeader("X-User-Roles"), ", ")
		if slices.Contains[[]string](roles, "admin") {
			return forUser
		}
	}

	userId = c.GetHeader("X-UserId")
	if userId == "" {
		if c.GetHeader("Authorization") != "" {
			claims, err := jwt.Parse(c.GetHeader("Authorization"))
			if err != nil {
				return
			}
			userId = claims.Sub
			if userId == "" {
				userId = "dummy"
			}
		}
	}
	return
}
