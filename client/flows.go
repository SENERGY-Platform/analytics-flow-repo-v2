/*
 * Copyright 2025 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
)

func (c *Client) GetFlows(token string, userId string) (resp lib.FlowsResponse, code int, err error) {
	req, err := http.NewRequest(http.MethodGet, c.baseUrl+"/flow", nil)
	if err != nil {
		return resp, http.StatusBadRequest, err
	}
	return do[lib.FlowsResponse](req, token, userId)
}

func (c *Client) GetFlow(token string, userId string, id string) (flow lib.Flow, code int, err error) {
	req, err := http.NewRequest(http.MethodGet, c.baseUrl+"/flow/"+id, nil)
	if err != nil {
		return flow, http.StatusBadRequest, err
	}
	return do[lib.Flow](req, token, userId)
}

func (c *Client) CreateFlow(token string, userId string, flow lib.Flow) (code int, err error) {
	b, err := json.Marshal(flow)
	if err != nil {
		return http.StatusBadRequest, err
	}
	req, err := http.NewRequest(http.MethodPut, c.baseUrl+"/flow/", bytes.NewBuffer(b))
	if err != nil {
		return http.StatusBadRequest, err
	}
	_, code, err = doNoDecode(req, token, userId)
	return code, err
}

func (c *Client) UpdateFlow(token string, userId string, flow lib.Flow) (code int, err error) {
	if flow.Id == nil {
		return http.StatusBadRequest, fmt.Errorf("flow needs non-nil id")
	}
	b, err := json.Marshal(flow)
	if err != nil {
		return http.StatusBadRequest, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseUrl+"/flow/"+flow.Id.Hex(), bytes.NewBuffer(b))
	if err != nil {
		return http.StatusBadRequest, err
	}
	_, code, err = doNoDecode(req, token, userId)
	return code, err
}

func (c *Client) DeleteFlow(token string, userId string, id string) (code int, err error) {
	req, err := http.NewRequest(http.MethodDelete, c.baseUrl+"/flow/"+id, nil)
	if err != nil {
		return http.StatusBadRequest, err
	}
	_, code, err = doNoDecode(req, token, userId)
	return code, err
}
