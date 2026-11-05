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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
)

// ownerRepo answers the owner route only; any other method panics through the nil interface.
type ownerRepo struct {
	Repo
	flow   lib.Flow
	err    error
	called bool
	id     string
	owner  string
}

func (o *ownerRepo) SetFlowOwner(id, owner string) (lib.Flow, error) {
	o.called, o.id, o.owner = true, id, owner
	return o.flow, o.err
}

func ownerRequest(t *testing.T, srv Repo, method string, headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()
	engine, err := New(srv, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, "/flow/flow-1/owner", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestOwnerAnswersTheMovedFlow(t *testing.T) {
	srv := &ownerRepo{flow: lib.Flow{Name: "heating", UserId: "user-c", DateUpdated: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}

	rec := ownerRequest(t, srv, http.MethodPut, adminHeaders, `{"owner": "user-c"}`)

	if rec.Code != http.StatusOK || srv.id != "flow-1" || srv.owner != "user-c" {
		t.Fatalf("status = %d, id %q, owner %q", rec.Code, srv.id, srv.owner)
	}
	if !strings.Contains(rec.Body.String(), `"userId":"user-c"`) || !strings.Contains(rec.Body.String(), `"dateUpdated":"2026-01-02T03:04:05Z"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestOwnerByANonAdminIs403AndTouchesNothing(t *testing.T) {
	for name, headers := range map[string]map[string]string{
		"user":         userHeaders,
		"no roles":     {"X-UserId": "user-a"},
		"for_user too": {"X-UserId": "user-a", "X-User-Roles": "user"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := &ownerRepo{}

			rec := ownerRequest(t, srv, http.MethodPut, headers, `{"owner": "user-c"}`)

			if rec.Code != http.StatusForbidden || srv.called {
				t.Errorf("status = %d, called = %v, want 403 and no call", rec.Code, srv.called)
			}
		})
	}
}

func TestOwnerWithABodyThatIsNoJsonIs400(t *testing.T) {
	srv := &ownerRepo{}

	rec := ownerRequest(t, srv, http.MethodPut, adminHeaders, `user-c`)

	if rec.Code != http.StatusBadRequest || srv.called {
		t.Errorf("status = %d, called = %v, want 400 and no call", rec.Code, srv.called)
	}
}

func TestOwnerPassesAMissingOwnerOnAsEmpty(t *testing.T) {
	srv := &ownerRepo{err: lib.NewInputError(errors.New("owner must be a user id"))}

	rec := ownerRequest(t, srv, http.MethodPut, adminHeaders, `{}`)

	if rec.Code != http.StatusBadRequest || !srv.called || srv.owner != "" {
		t.Errorf("status = %d, called = %v, owner %q", rec.Code, srv.called, srv.owner)
	}
}

func TestOwnerAnswersRefusalsWithTheirStatus(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{lib.NewNotFoundError(errors.New("could not find flow")), http.StatusNotFound},
		{errors.New("permissions-v2 down"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		srv := &ownerRepo{err: tc.err}

		rec := ownerRequest(t, srv, http.MethodPut, adminHeaders, `{"owner": "user-c"}`)

		if rec.Code != tc.code || strings.Contains(rec.Body.String(), "permissions-v2") {
			t.Errorf("status = %d, body %q, want %d without internals", rec.Code, rec.Body.String(), tc.code)
		}
	}
}

// Clients probe the route with a GET and read 405 as "route exists".
func TestOwnerPathAnswers405ToAGet(t *testing.T) {
	srv := &ownerRepo{}

	rec := ownerRequest(t, srv, http.MethodGet, adminHeaders, "")

	if rec.Code != http.StatusMethodNotAllowed || srv.called {
		t.Errorf("status = %d, called = %v, want 405", rec.Code, srv.called)
	}
}

// The new route must not change what the admin group answers to a non-admin.
func TestAdminRoutesStillAnswer401ToANonAdmin(t *testing.T) {
	engine, err := New(&ownerRepo{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/statistics/operator-usage", nil)
	req.Header.Set("X-UserId", "user-a")
	req.Header.Set("X-User-Roles", "user")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
