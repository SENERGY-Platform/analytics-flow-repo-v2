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

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
)

// writeRepo answers create and update only; any other method panics through the nil interface.
type writeRepo struct {
	Repo
	err error
}

func (w *writeRepo) CreateFlow(lib.Flow, string, string) (string, error) { return "", w.err }

func (w *writeRepo) UpdateFlow(string, lib.Flow, string, string) error { return w.err }

func TestWriteFlowAnswersAnInputErrorWith400AndItsText(t *testing.T) {
	srv := &writeRepo{err: lib.NewInputError(errors.New("node n1 pins operator version v9.9.9, which the operator does not have"))}
	engine, err := New(srv, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ method, path string }{{http.MethodPut, "/flow/"}, {http.MethodPost, "/flow/abc/"}} {
		req := httptest.NewRequest(target.method, target.path, strings.NewReader(`{"name":"f"}`))
		req.Header.Set("X-UserId", "user-a")
		rec := httptest.NewRecorder()

		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s: status = %d, want 400", target.method, target.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "n1") || !strings.Contains(rec.Body.String(), "v9.9.9") {
			t.Errorf("%s %s: body %q does not name the node and the label", target.method, target.path, rec.Body.String())
		}
	}
}
