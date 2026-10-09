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
	"strconv"
	"strings"

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
// @Accept json
// @Produce json
// @Success	201 {object} lib.FlowCreateResponse
// @Failure 400 {string} MessageBadInput
// @Failure 401 {string} MessageUnauthorized
// @Failure 424 {string} MessageExternalResourceError
// @Failure 500 {string} MessageSomethingWrong
// @Router /flow/ [put]
func putFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodPut, FlowPath + "/", func(gc *gin.Context) {
		var request lib.Flow
		if err := gc.ShouldBindJSON(&request); err != nil {
			util.Logger.Error("error creating flow", "error", err)
			_ = gc.Error(lib.NewInputError(errors.New(MessageBadInput)))
			return
		}
		id, err := srv.CreateFlow(request, gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error creating flow", "error", err)
			_ = gc.Error(handleError(err))
			return
		}
		gc.JSON(http.StatusCreated, lib.FlowCreateResponse{Id: id})
	}
}

// postFlow godoc
// @Summary Update flow
// @Description	Validates and updates a flow
// @Tags Flow
// @Accept json
// @Param id path string true "Flow ID"
// @Param flow body lib.Flow	true "Update flow"
// @Success	200
// @Failure 400 {string} MessageBadInput
// @Failure 401 {string} MessageUnauthorized
// @Failure 403 {string} MessageForbidden
// @Failure 404 {string} MessageNotFound
// @Failure 424 {string} MessageExternalResourceError
// @Failure 500 {string} MessageSomethingWrong
// @Router /flow/{id}/ [post]
func postFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodPost, FlowPath + "/:id/", func(gc *gin.Context) {
		var request lib.Flow
		if err := gc.ShouldBindJSON(&request); err != nil {
			util.Logger.Error("error updating flow", "error", err)
			_ = gc.Error(lib.NewInputError(errors.New(MessageBadInput)))
			return
		}
		err := srv.UpdateFlow(gc.Param("id"), request, gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error updating flow", "error", err)
			_ = gc.Error(handleError(err))
			return
		}
		gc.Status(http.StatusOK)
	}
}

// deleteFlow godoc
// @Summary Delete flow
// @Description	Deletes a flow. Refused with 409 while pipelines or smart services use it, unless an administrator passes force. Answers 424 when the pipeline registry cannot be asked and 502 when the smart-service-repository cannot; nothing is deleted then, also with force.
// @Tags Flow
// @Produce json
// @Param id path string true "Flow ID"
// @Param force query bool false "Administrators only: delete although pipelines or smart services still use the flow" default(false)
// @Success	204
// @Failure 400 {string} MessageBadInput
// @Failure 401 {string} MessageUnauthorized
// @Failure 403 {string} MessageForbidden
// @Failure 404 {string} MessageNotFound
// @Failure	409 {object} lib.StillInUseResponse
// @Failure 424 {string} MessageExternalResourceError
// @Failure 500 {string} MessageSomethingWrong
// @Failure 502 {string} MessageUsageUnavailable
// @Router /flow/{id}/ [delete]
func deleteFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodDelete, FlowPath + "/:id/", func(gc *gin.Context) {
		opts, err := deleteOptions(gc)
		if err != nil {
			_ = gc.Error(err)
			return
		}
		err = srv.DeleteFlow(gc.Param("id"), gc.GetString(UserIdKey), gc.GetHeader("Authorization"), opts)
		if err != nil {
			util.Logger.Error("error deleting flow", "error", err)
			err = handleError(err)
			var inUse *lib.StillInUseError
			if errors.As(err, &inUse) {
				// The error handler would reduce the usage to text.
				gc.JSON(http.StatusConflict, stillInUseResponse(inUse))
				return
			}
			_ = gc.Error(err)
			return
		}
		gc.Status(http.StatusNoContent)
	}
}

// deleteOptions reads ?force from the request. Force skips the refusal for pipelines and smart
// services that use the flow, so it is for administrators only.
func deleteOptions(gc *gin.Context) (lib.DeleteOptions, error) {
	raw := gc.Query("force")
	if raw == "" {
		return lib.DeleteOptions{}, nil
	}
	force, err := strconv.ParseBool(raw)
	if err != nil {
		return lib.DeleteOptions{}, lib.NewInputError(errors.New(MessageBadInput))
	}
	if force {
		admin, err := isAdmin(gc)
		if err != nil {
			util.Logger.Warn("could not check the admin role", "error", err)
		}
		if err != nil || !admin {
			return lib.DeleteOptions{}, lib.NewForbiddenError(errors.New(MessageForbidden))
		}
	}
	return lib.DeleteOptions{Force: force}, nil
}

func stillInUseResponse(e *lib.StillInUseError) lib.StillInUseResponse {
	body := lib.StillInUseResponse{Error: e.Error(), Pipelines: lib.PipelineCount(e.FlowUsage), Readable: []lib.SmartServiceRef{}}
	if e.Smart != nil {
		body.Releases, body.Instances = e.Smart.Releases, e.Smart.Instances
		if e.Smart.Readable != nil {
			body.Readable = e.Smart.Readable
		}
	}
	return body
}

// getAll godoc
// @Summary Get flows
// @Description	Gets all flows
// @Tags Flow
// @Produce json
// @Success	200 {object} lib.FlowsResponse
// @Failure 401 {string} MessageUnauthorized
// @Failure 500 {string} MessageSomethingWrong
// @Router /flow [get]
func getAll(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, FlowPath, func(gc *gin.Context) {
		args := gc.Request.URL.Query()
		flows, err := srv.GetFlows(gc.GetString(UserIdKey), args, gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error getting flows", "error", err)
			_ = gc.Error(handleError(err))
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
// @Failure 401 {string} MessageUnauthorized
// @Failure 403 {string} MessageForbidden
// @Failure 404 {string} MessageNotFound
// @Failure 500 {string} MessageSomethingWrong
// @Router /flow/{id} [get]
func getFlow(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/flow/:id", func(gc *gin.Context) {
		flow, err := srv.GetFlow(gc.Param("id"), gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error getting flow", "error", err)
			_ = gc.Error(handleError(err))
			return
		}
		gc.JSON(http.StatusOK, flow)
	}
}

// getOperatorUsage godoc
// @Summary Get usage of an operator
// @Description	Counts the flows of all users that contain a node of the operator and lists the ones the caller may read.
// @Tags Operator
// @Produce json
// @Param id path string true "Operator ID"
// @Success	200 {object} lib.OperatorFlowUsage
// @Failure 400 {string} MessageBadInput
// @Failure 401 {string} MessageUnauthorized
// @Failure 500 {string} MessageSomethingWrong
// @Router /operators/{id}/usage [get]
func getOperatorUsage(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, OperatorPath + "/:id/usage", func(gc *gin.Context) {
		id := gc.Param("id")
		// The flow filter splits on these two characters, so such an id would match other operators.
		if strings.ContainsAny(id, ",|") {
			_ = gc.Error(lib.NewInputError(errors.New(MessageBadInput)))
			return
		}
		usage, err := srv.GetUsageOfOperator(id, gc.GetString(UserIdKey), gc.GetHeader("Authorization"))
		if err != nil {
			util.Logger.Error("error getting operator usage", "error", err)
			_ = gc.Error(handleError(err))
			return
		}
		gc.JSON(http.StatusOK, usage)
	}
}

func getOperatorUsageAdmin(srv Repo) (string, string, gin.HandlerFunc) {
	return http.MethodGet, "/admin/statistics/operator-usage", func(gc *gin.Context) {
		data, err := srv.GetOperatorUsage()
		if err != nil {
			util.Logger.Error("error getting operator usage", "error", err)
			_ = gc.Error(handleError(err))
			return
		}
		gc.JSON(http.StatusOK, data)
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
