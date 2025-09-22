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
	"context"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/models"
	srv_info_hdl "github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
)

type Repo interface {
	SrvInfo(ctx context.Context) srv_info_hdl.ServiceInfo
	HealthCheck(ctx context.Context) error
	CreateFlow(flow models.Flow, userId string, authString string) (err error)
	UpdateFlow(id string, flow models.Flow, userId string, authString string) (err error)
	DeleteFlow(id string, userId string, auth string) (err error)
	GetFlows(userId string, args map[string][]string, auth string) (response models.FlowsResponse, err error)
	GetFlow(flowId string, userId string, auth string) (response models.Flow, err error)
}
