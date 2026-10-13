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

package operator_api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOperatorReturnsAnErrorWhenTheServiceIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	_, err := New(url).GetOperator("op-1", "user-a", "Bearer a")

	if err == nil {
		t.Fatal("want an error for an unreachable operator service")
	}
}

func TestGetOperatorMapsAStatusErrorToAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	if _, err := New(srv.URL).GetOperator("op-1", "user-a", ""); err == nil {
		t.Fatal("want an error for a 404")
	}
}

func TestGetOperatorReadsVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/operator/op-1" || r.Header.Get("X-UserId") != "user-a" || r.Header.Get("Authorization") != "Bearer a" {
			t.Errorf("unexpected request %s with %q, %q", r.URL.Path, r.Header.Get("X-UserId"), r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"name":"filter","image":"i:2","versions":[{"version":"v2.0.0","image":"i:2","deprecated":false}]}`))
	}))
	defer srv.Close()

	op, err := New(srv.URL).GetOperator("op-1", "user-a", "Bearer a")

	if err != nil {
		t.Fatal(err)
	}
	if len(op.Versions) != 1 || op.Versions[0].Version != "v2.0.0" || op.Versions[0].Image != "i:2" {
		t.Errorf("versions = %+v", op.Versions)
	}
}
