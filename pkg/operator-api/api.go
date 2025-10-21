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

package operator_api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	operator_repo "github.com/SENERGY-Platform/analytics-operator-repo-v2/lib"
	"github.com/parnurzeal/gorequest"
)

type Repo struct {
	url string
}

func New(url string) *Repo {
	return &Repo{url}
}

func (a Repo) GetOperator(id string, userId string, authorization string) (o operator_repo.Operator, err error) {
	request := gorequest.New()
	request.Get(a.url+"/operator/"+id).Set("X-UserId", userId).Set("Authorization", authorization)
	resp, body, e := request.End()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("operator API - could not get operator from operator service: " + strconv.Itoa(resp.StatusCode) + " " + body)
		return
	}
	if len(e) > 0 {
		err = errors.New("operator API - could not get operator from operator service: an error occurred")
		return
	}
	err = json.Unmarshal([]byte(body), &o)
	return
}
