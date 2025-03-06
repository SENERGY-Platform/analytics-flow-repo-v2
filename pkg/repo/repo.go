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

package repo

import (
	"context"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/models"
	srv_info_hdl "github.com/SENERGY-Platform/mgw-go-service-base/srv-info-hdl"
	srv_info_lib "github.com/SENERGY-Platform/mgw-go-service-base/srv-info-hdl/lib"
)

type Repo struct {
	srvInfoHdl srv_info_hdl.SrvInfoHandler
	dbRepo     FlowRepository
}

func New(srvInfoHdl srv_info_hdl.SrvInfoHandler) *Repo {
	return &Repo{
		srvInfoHdl: srvInfoHdl,
		dbRepo:     NewMongoRepo(),
	}
}

func (r *Repo) SrvInfo(_ context.Context) srv_info_lib.SrvInfo {
	return r.srvInfoHdl.GetInfo()
}

func (r *Repo) HealthCheck(ctx context.Context) error {
	return nil
}

func (r *Repo) GetFlows(userId string, args map[string][]string) (response models.FlowsResponse, err error) {
	return r.dbRepo.All(userId, false, args)
}
