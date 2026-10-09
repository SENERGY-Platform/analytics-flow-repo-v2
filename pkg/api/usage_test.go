/*
 * Copyright 2026 InfAI (CC SES)
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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
)

func TestMain(m *testing.M) {
	util.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	os.Exit(m.Run())
}

// usageRepo answers the usage route only; any other method panics through the nil interface.
type usageRepo struct {
	Repo
	usage      lib.OperatorFlowUsage
	err        error
	calls      int
	operatorId string
	userId     string
	auth       string
}

func (u *usageRepo) GetUsageOfOperator(operatorId, userId, auth string) (lib.OperatorFlowUsage, error) {
	u.calls++
	u.operatorId, u.userId, u.auth = operatorId, userId, auth
	return u.usage, u.err
}

func getUsage(t *testing.T, srv Repo, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	engine, err := New(srv, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestGetOperatorUsageRequiresAToken(t *testing.T) {
	srv := &usageRepo{}

	rec := getUsage(t, srv, "/operators/op-1/usage", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if srv.calls != 0 {
		t.Error("the repo was asked without an identity")
	}
}

func TestGetOperatorUsageAnswersForAnyAuthenticatedUser(t *testing.T) {
	srv := &usageRepo{usage: lib.OperatorFlowUsage{Flows: 3, Readable: []lib.FlowRef{{Id: "f1", Name: "flow one"}}}}

	// A plain user: no admin role anywhere in the request.
	rec := getUsage(t, srv, "/operators/op-1/usage", map[string]string{
		"X-UserId":      "user-a",
		"X-User-Roles":  "user",
		"Authorization": "Bearer token-a",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["flows"] != float64(3) {
		t.Errorf("flows = %v, want 3", body["flows"])
	}
	readable, _ := body["readable"].([]any)
	if len(readable) != 1 || readable[0].(map[string]any)["id"] != "f1" || readable[0].(map[string]any)["name"] != "flow one" {
		t.Errorf("readable = %v", body["readable"])
	}
	if srv.operatorId != "op-1" || srv.userId != "user-a" || srv.auth != "Bearer token-a" {
		t.Errorf("repo called with %q, %q, %q", srv.operatorId, srv.userId, srv.auth)
	}
}

func TestGetOperatorUsageOfAnUnusedOperatorIsAnEmptyList(t *testing.T) {
	srv := &usageRepo{usage: lib.OperatorFlowUsage{Readable: []lib.FlowRef{}}}

	rec := getUsage(t, srv, "/operators/op-1/usage", map[string]string{"X-UserId": "user-a"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got, want := rec.Body.String(), `{"flows":0,"readable":[]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestGetOperatorUsageRejectsIdsTheFilterWouldSplit(t *testing.T) {
	for _, id := range []string{"a,b", "a%7Cb"} {
		srv := &usageRepo{}

		rec := getUsage(t, srv, "/operators/"+id+"/usage", map[string]string{"X-UserId": "user-a"})

		if rec.Code != http.StatusBadRequest {
			t.Errorf("id %q: status = %d, want 400", id, rec.Code)
		}
		if srv.calls != 0 {
			t.Errorf("id %q: the repo was asked", id)
		}
	}
}

func TestGetOperatorUsageHidesInternalErrors(t *testing.T) {
	srv := &usageRepo{err: errors.New("mongo: auth error for user svc")}

	rec := getUsage(t, srv, "/operators/op-1/usage", map[string]string{"X-UserId": "user-a"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); body != MessageSomethingWrong {
		t.Errorf("body = %q, want %q", body, MessageSomethingWrong)
	}
}
