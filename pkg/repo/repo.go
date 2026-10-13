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
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	operator_api "github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/operator-api"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/smartservices"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	operator_repo "github.com/SENERGY-Platform/analytics-operator-repo-v2/lib"
	pipelinesClient "github.com/SENERGY-Platform/analytics-pipeline/client"
	pipelinesLib "github.com/SENERGY-Platform/analytics-pipeline/lib"
	srv_info_hdl "github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
)

type Repo struct {
	srvInfoHdl   srv_info_hdl.Handler
	dbRepo       FlowRepository
	operatorRepo OperatorGetter
	pipe         pipelinesClient.Client
	smart        SmartServiceUsage
}

// OperatorGetter reads an operator with the caller's identity; *operator_api.Repo implements it.
type OperatorGetter interface {
	GetOperator(id, userId, authorization string) (operator_repo.Operator, error)
}

// SmartServiceUsage asks which smart services use a flow, with the caller's token.
type SmartServiceUsage interface {
	Usage(ctx context.Context, flowId string, auth string) (lib.SmartServiceUsage, error)
}

func New(srvInfoHdl srv_info_hdl.Handler, perm permV2Client.Client, operatorRepo *operator_api.Repo, pipe pipelinesClient.Client, smartServiceRepoUrl string) (*Repo, error) {
	if smartServiceRepoUrl == "" {
		// Without it every delete would end in a 502; better to fail at startup.
		return nil, errors.New("no smart-service-repository address configured")
	}
	dbRepo := NewMongoRepo(perm)
	err := dbRepo.validateFlowPermissions()
	return &Repo{
		srvInfoHdl:   srvInfoHdl,
		dbRepo:       dbRepo,
		operatorRepo: operatorRepo,
		pipe:         pipe,
		smart:        smartservices.New(smartServiceRepoUrl),
	}, err
}

func (r *Repo) SrvInfo(_ context.Context) srv_info_hdl.ServiceInfo {
	return r.srvInfoHdl.ServiceInfo()
}

func (r *Repo) HealthCheck(_ context.Context) error {
	return nil
}

func (r *Repo) CreateFlow(flow lib.Flow, userId string, auth string) (id string, err error) {
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

// validateOperators fills name, deployment type, cost and image of every operator node from the
// operator service. A node that pins a version gets the image of that version; ports and config
// stay as the client sent them.
func (r *Repo) validateOperators(flow *lib.Flow, userId string, auth string) error {
	for i, operator := range flow.Model.Cells {
		if operator.Type == "senergy.NodeElement" {
			if operator.OperatorId == nil || *operator.OperatorId == "" {
				return lib.NewInputError(fmt.Errorf("node %s has no operatorId", operator.Id))
			}
			op, err := r.operatorRepo.GetOperator(*operator.OperatorId, userId, auth)
			if err != nil {
				return lib.NewExternalResourceError(err)
			}
			image := op.Image
			if operator.OperatorVersion != nil && *operator.OperatorVersion == "" {
				// An empty pin means no pin; storing it would hand clients a key they do not expect.
				operator.OperatorVersion = nil
			}
			if operator.OperatorVersion != nil {
				label := *operator.OperatorVersion
				idx := slices.IndexFunc(op.Versions, func(v operator_repo.OperatorVersion) bool { return v.Version == label })
				if idx < 0 {
					return lib.NewInputError(fmt.Errorf("node %s pins operator version %s, which the operator does not have", operator.Id, label))
				}
				image = op.Versions[idx].Image
			}
			operator.Name = &op.Name
			operator.Image = &image
			operator.DeploymentType = &op.DeploymentType
			if op.Cost != nil {
				operator.Cost = op.Cost
			}
			flow.Model.Cells[i] = operator
		}
	}
	return nil
}

// DeleteFlow refuses with a *lib.StillInUseError while pipelines or smart services use the flow,
// unless opts.Force is set. An unknown answer of either service never deletes, also with force:
// the pipeline registry gives a *lib.ExternalResourceError, the smart-service-repository a
// *lib.UsageUnavailableError. Check and delete are not atomic: a pipeline or release created in
// between is not seen.
func (r *Repo) DeleteFlow(id, userId, auth string, opts lib.DeleteOptions) (err error) {
	usage, err, code := r.pipe.GetFlowUsageById(auth, userId, id)
	if err != nil {
		return lib.NewExternalResourceError(err)
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return lib.NewExternalResourceError(errors.New("pipeline registry error, wrong status code " + strconv.Itoa(code)))
	}
	// Nil unless pipelines use the flow; a 200 without a body still says that they do.
	var pipelines *pipelinesLib.FlowUsage
	if code == http.StatusOK {
		pipelines = usage
		if pipelines == nil {
			pipelines = &pipelinesLib.FlowUsage{}
		}
	}

	smart, err := r.smart.Usage(context.Background(), id, auth)
	if err != nil {
		// The detail may hold internals and the handler logs the error at error level.
		util.Logger.Warn("could not ask the smart-service-repository for the usage of a flow", "flow_id", id, "error", err)
		return lib.NewUsageUnavailableError(errors.New("could not check whether smart services use the flow, nothing was deleted"))
	}
	var smartUse *lib.SmartServiceUsage
	if smart.Releases > 0 {
		smartUse = &smart
	}

	used := pipelines != nil || smartUse != nil
	if used && !opts.Force {
		return lib.NewStillInUseError(pipelines, smartUse, errors.New("flow still in use"))
	}

	if err = r.dbRepo.DeleteFlow(id, userId, false, auth); err != nil {
		return err
	}
	if used {
		util.Logger.Warn("deleted a flow that pipelines or smart services still use",
			"flow_id", id, "user_id", userId, "pipelines", lib.PipelineCount(pipelines),
			"releases", smart.Releases, "instances", smart.Instances)
	}
	return nil
}

func (r *Repo) GetFlows(userId string, args map[string][]string, auth string) (response lib.FlowsResponse, err error) {
	return r.dbRepo.All(userId, false, args, auth)
}

func (r *Repo) GetFlow(flowId, userId, auth string) (response lib.Flow, err error) {
	return r.dbRepo.FindFlow(flowId, userId, auth)
}

func (r *Repo) GetOperatorUsage() ([]lib.OperatorFlowCount, error) {
	return r.dbRepo.GetOperatorFlowMapping("")
}

// GetUsageOfOperator counts the flows of all users from the same aggregation as GetOperatorUsage and
// lists the flows the caller may read through the flow listing, so both use one read check. A
// non-empty version restricts both to flows with a node of the operator that pins that label; it
// must be a valid label, which cannot contain the characters the listing filter splits on.
func (r *Repo) GetUsageOfOperator(operatorId, version, userId, auth string) (usage lib.OperatorFlowUsage, err error) {
	mapping, err := r.dbRepo.GetOperatorFlowMapping(version)
	if err != nil {
		return
	}
	for _, m := range mapping {
		if m.OperatorID == operatorId {
			usage.Flows = len(m.Flows)
			break
		}
	}
	usage.Readable = []lib.FlowRef{}
	if usage.Flows == 0 {
		return
	}
	// The filter splits on "," and "|"; the handler rejects ids containing them.
	filter := "operator:" + operatorId
	if version != "" {
		filter += "|operatorversion:" + version
	}
	readable, err := r.dbRepo.All(userId, false, map[string][]string{"filter": {filter}}, auth)
	if err != nil {
		return lib.OperatorFlowUsage{}, err
	}
	for _, flow := range readable.Flows {
		ref := lib.FlowRef{Name: flow.Name}
		if flow.Id != nil {
			ref.Id = flow.Id.Hex()
		}
		usage.Readable = append(usage.Readable, ref)
	}
	return
}
