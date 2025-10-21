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

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	operator_api "github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/operator-api"
	srv_info_hdl "github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
)

type Repo struct {
	srvInfoHdl   srv_info_hdl.Handler
	dbRepo       FlowRepository
	operatorRepo *operator_api.Repo
}

func New(srvInfoHdl srv_info_hdl.Handler, perm permV2Client.Client, operatorRepo *operator_api.Repo) (*Repo, error) {
	dbRepo := NewMongoRepo(perm)
	err := dbRepo.validateFlowPermissions()
	return &Repo{
		srvInfoHdl:   srvInfoHdl,
		dbRepo:       dbRepo,
		operatorRepo: operatorRepo,
	}, err
}

func (r *Repo) SrvInfo(_ context.Context) srv_info_hdl.ServiceInfo {
	return r.srvInfoHdl.ServiceInfo()
}

func (r *Repo) HealthCheck(ctx context.Context) error {
	return nil
}

func (r *Repo) CreateFlow(flow lib.Flow, userId string, auth string) (err error) {
	err = r.validateOperators(&flow, userId, auth)
	if err != nil {
		return
	}
	flow.UserId = userId
	return r.dbRepo.InsertFlow(flow)
}

func (r *Repo) UpdateFlow(id string, flow lib.Flow, userId string, auth string) (err error) {
	err = r.validateOperators(&flow, userId, auth)
	if err != nil {
		return
	}
	return r.dbRepo.UpdateFlow(id, flow, userId, auth)
}

func (r *Repo) validateOperators(flow *lib.Flow, userId string, auth string) error {
	for i, operator := range flow.Model.Cells {
		if operator.Type == "senergy.NodeElement" {
			op, err := r.operatorRepo.GetOperator(*operator.OperatorId, userId, auth)
			if err != nil {
				return err
			}
			operator.Name = &op.Name
			operator.Image = &op.Image
			operator.DeploymentType = &op.DeploymentType
			if op.Cost != nil {
				operator.Cost = op.Cost
			}
			flow.Model.Cells[i] = operator
		}
	}
	return nil
}

func (r *Repo) DeleteFlow(id string, userId string, auth string) (err error) {
	return r.dbRepo.DeleteFlow(id, userId, false, auth)
}

func (r *Repo) GetFlows(userId string, args map[string][]string, auth string) (response lib.FlowsResponse, err error) {
	return r.dbRepo.All(userId, false, args, auth)
}

func (r *Repo) GetFlow(flowId string, userId string, auth string) (response lib.Flow, err error) {
	return r.dbRepo.FindFlow(flowId, userId, auth)
}
