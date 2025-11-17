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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseUrl string
}

func NewClient(baseUrl string) *Client {
	return &Client{baseUrl: baseUrl}
}

func doNoDecode(req *http.Request, token string, userId string) (resp *http.Response, code int, err error) {
	req.Header.Set("Authorization", token)
	req.Header.Set("X-UserId", userId)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return resp, http.StatusInternalServerError, err
	}
	if resp.StatusCode > 299 {
		defer resp.Body.Close()
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return resp, resp.StatusCode, fmt.Errorf("unexpected statuscode %v: %v", resp.StatusCode, string(temp))
	}
	return
}

func do[T any](req *http.Request, token string, userId string) (result T, code int, err error) {
	resp, code, err := doNoDecode(req, token, userId)
	if err != nil {
		return result, code, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		_, _ = io.ReadAll(resp.Body) //ensure resp.Body is read to EOF
		return result, http.StatusInternalServerError, err
	}
	return
}
