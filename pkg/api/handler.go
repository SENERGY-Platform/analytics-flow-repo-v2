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
	"errors"
	"net/http"
	"os"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"

	"github.com/gin-gonic/gin"
)

// getInfoH godoc
// @Summary Get service info
// @Description	Get basic service and runtime information.
// @Tags Info
// @Produce	json
// @Success	200 {object} srv_info_hdl.ServiceInfo "info"
// @Failure	500 {string} string "error message"
// @Router /info [get]
func getInfoH(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/info", func(gc *gin.Context) {
		gc.JSON(http.StatusOK, srv.SrvInfo(gc.Request.Context()))
	}
}

// putFlow godoc
// @Summary Create flow
// @Description	Validates and stores a flow
// @Tags Flow
// @Param flow body lib.Flow	true "Create flow"
// @Accept       json
// @Success	201
// @Failure	500 {string} str
// @Router /flow/ [put]
func putFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodPut, "/flow/", func(gc *gin.Context) {
		var request lib.Flow
		if err := gc.ShouldBindJSON(&request); err != nil {
			util.Logger.Error("error creating flow", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
			return
		}
		err := srv.CreateFlow(request, gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error creating flow", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
			return
		}
		gc.Status(http.StatusCreated)
	}
}

// postFlow godoc
// @Summary Update flow
// @Description	Validates and updates a flow
// @Tags Flow
// @Accept       json
// @Param id path string true "Flow ID"
// @Param flow body lib.Flow	true "Update flow"
// @Success	200
// @Failure	500 {string} str
// @Router /flow/{id}/ [post]
func postFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodPost, "/flow/:id/", func(gc *gin.Context) {
		var request lib.Flow
		if err := gc.ShouldBindJSON(&request); err != nil {
			util.Logger.Error("error updating flow", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
			return
		}
		err := srv.UpdateFlow(gc.Param("id"), request, gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error updating flow", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
			return
		}
		gc.Status(http.StatusOK)
	}
}

// deleteFlow godoc
// @Summary Delete flow
// @Description	Deletes a flow
// @Tags Flow
// @Param id path string true "Flow ID"
// @Success	204
// @Failure	500 {string} str
// @Router /flow/{id}/ [delete]
func deleteFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, "/flow/:id/", func(gc *gin.Context) {
		err := srv.DeleteFlow(gc.Param("id"), gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error deleting flow", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
			return
		}
		gc.Status(http.StatusNoContent)
	}
}

// getAll godoc
// @Summary Get flows
// @Description	Gets all flows
// @Tags Flow
// @Produce json
// @Success	200 {object} lib.FlowsResponse
// @Failure	500 {string} str
// @Router /flow [get]
func getAll(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/flow", func(gc *gin.Context) {
		args := gc.Request.URL.Query()
		flows, err := srv.GetFlows(gc.GetString(UserIdKey), args, gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error getting flows", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
			return
		}
		gc.JSON(http.StatusOK, flows)
	}
}

// getFlow godoc
// @Summary Get flow
// @Description	Gets a single flow
// @Tags Flow
// @Produce json
// @Param id path string true "Flow ID"
// @Success	200 {object} lib.Flow
// @Failure	500 {string} str
// @Router /flow/{id} [get]
func getFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/flow/:id", func(gc *gin.Context) {
		flow, err := srv.GetFlow(gc.Param("id"), gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error getting flow", "error", err)
			_ = gc.Error(errors.New(MessageSomethingWrong))
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
