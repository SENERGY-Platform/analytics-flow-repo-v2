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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	pipelinesLib "github.com/SENERGY-Platform/analytics-pipeline/lib"
)

// deleteRepo answers the delete route only; any other method panics through the nil interface.
type deleteRepo struct {
	Repo
	err    error
	called bool
	id     string
	auth   string
	opts   lib.DeleteOptions
}

func (d *deleteRepo) DeleteFlow(id, _, auth string, opts lib.DeleteOptions) error {
	d.called, d.id, d.auth, d.opts = true, id, auth, opts
	return d.err
}

func deleteFlowRequest(t *testing.T, srv Repo, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	engine, err := New(srv, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodDelete, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

var (
	userHeaders  = map[string]string{"X-UserId": "user-a", "X-User-Roles": "user", "Authorization": "Bearer token-a"}
	adminHeaders = map[string]string{"X-UserId": "admin-a", "X-User-Roles": "user, admin", "Authorization": "Bearer token-a"}
)

func TestDeleteFlowAnswersARefusalWithBothUsages(t *testing.T) {
	srv := &deleteRepo{err: lib.NewStillInUseError(
		&pipelinesLib.FlowUsage{FlowId: "flow-1", Count: 2, PipelineIds: []string{"p1", "p2"}},
		&lib.SmartServiceUsage{Releases: 3, Instances: 7, Readable: []lib.SmartServiceRef{{Id: "r1", DesignId: "d1", Name: "heating"}}},
		errors.New("flow still in use"),
	)}

	rec := deleteFlowRequest(t, srv, "/flow/flow-1/", userHeaders)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	if body["error"] != MessageStillInUse || body["pipelines"] != float64(2) || body["releases"] != float64(3) || body["instances"] != float64(7) {
		t.Errorf("body = %v", body)
	}
	readable, _ := body["readable"].([]any)
	if len(readable) != 1 || readable[0].(map[string]any)["id"] != "r1" || readable[0].(map[string]any)["design_id"] != "d1" || readable[0].(map[string]any)["name"] != "heating" {
		t.Errorf("readable = %v", body["readable"])
	}
	if _, leaked := body["pipelineIds"]; leaked {
		t.Error("the ids of pipelines, which may belong to other users, are in the body")
	}
	if srv.id != "flow-1" || srv.auth != "Bearer token-a" || srv.opts.Force {
		t.Errorf("repo called with %q, %q, %+v", srv.id, srv.auth, srv.opts)
	}
}

// A refusal for pipelines alone keeps its status and message and gains zero counts.
func TestDeleteFlowAnswersAPipelineOnlyRefusalWithZeroReleases(t *testing.T) {
	srv := &deleteRepo{err: lib.NewStillInUseError(&pipelinesLib.FlowUsage{Count: 1}, nil, errors.New("flow still in use"))}

	rec := deleteFlowRequest(t, srv, "/flow/flow-1/", userHeaders)

	if got, want := rec.Body.String(), `{"error":"still in use","pipelines":1,"releases":0,"instances":0,"readable":[]}`; rec.Code != http.StatusConflict || got != want {
		t.Errorf("status = %d, body = %s, want 409 and %s", rec.Code, got, want)
	}
}

func TestDeleteFlowAnswersAnUnknownUsageWith502(t *testing.T) {
	srv := &deleteRepo{err: lib.NewUsageUnavailableError(errors.New("dial tcp 10.0.0.1: refused"))}

	rec := deleteFlowRequest(t, srv, "/flow/flow-1/", userHeaders)

	if rec.Code != http.StatusBadGateway || rec.Body.String() != MessageUsageUnavailable {
		t.Errorf("status = %d, body = %q, want 502 and the static message", rec.Code, rec.Body.String())
	}
}

func TestDeleteFlowOfAnUnusedFlowIs204(t *testing.T) {
	srv := &deleteRepo{}

	rec := deleteFlowRequest(t, srv, "/flow/flow-1/", userHeaders)

	if rec.Code != http.StatusNoContent || !srv.called {
		t.Errorf("status = %d, called = %v", rec.Code, srv.called)
	}
}

func TestDeleteFlowForce(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		headers   map[string]string
		wantCode  int
		wantForce bool
		wantCall  bool
	}{
		{"no force", "/flow/f/", userHeaders, http.StatusNoContent, false, true},
		{"force false by a user", "/flow/f/?force=false", userHeaders, http.StatusNoContent, false, true},
		{"force true by an admin", "/flow/f/?force=true", adminHeaders, http.StatusNoContent, true, true},
		{"force true by a user", "/flow/f/?force=true", userHeaders, http.StatusForbidden, false, false},
		{"force true without any role", "/flow/f/?force=true", map[string]string{"X-UserId": "user-a"}, http.StatusForbidden, false, false},
		{"force that is no boolean", "/flow/f/?force=maybe", adminHeaders, http.StatusBadRequest, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := &deleteRepo{}

			rec := deleteFlowRequest(t, srv, tc.target, tc.headers)

			if rec.Code != tc.wantCode || srv.called != tc.wantCall || srv.opts.Force != tc.wantForce {
				t.Errorf("status = %d, called = %v, force = %v, want %d, %v, %v", rec.Code, srv.called, srv.opts.Force, tc.wantCode, tc.wantCall, tc.wantForce)
			}
		})
	}
}
