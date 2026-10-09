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

package repo

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	pipelinesClient "github.com/SENERGY-Platform/analytics-pipeline/client"
	srv_info_hdl "github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
)

type deleteDb struct {
	FlowRepository
	deleted []string
}

func (d *deleteDb) DeleteFlow(id string, _ string, _ bool, _ string) error {
	d.deleted = append(d.deleted, id)
	return nil
}

type fakeSmart struct {
	usage lib.SmartServiceUsage
	err   error
	asked int
	id    string
	auth  string
}

func (f *fakeSmart) Usage(_ context.Context, flowId string, auth string) (lib.SmartServiceUsage, error) {
	f.asked++
	f.id, f.auth = flowId, auth
	return f.usage, f.err
}

// registry answers the pipeline registry's flow usage route: 200 with the pipelines, 204 for none.
func registry(t *testing.T, status int, body string) *pipelinesClient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/pipeline/statistics/flowusage/") {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return pipelinesClient.NewClient(srv.URL)
}

type deleteFixture struct {
	repo  *Repo
	db    *deleteDb
	smart *fakeSmart
	logs  *bytes.Buffer
}

func newDeleteFixture(t *testing.T, registryStatus int, registryBody string, smart lib.SmartServiceUsage) *deleteFixture {
	t.Helper()
	f := &deleteFixture{db: &deleteDb{}, smart: &fakeSmart{usage: smart}, logs: &bytes.Buffer{}}
	f.repo = &Repo{dbRepo: f.db, pipe: *registry(t, registryStatus, registryBody), smart: f.smart}
	orig := util.Logger
	util.Logger = slog.New(slog.NewTextHandler(f.logs, nil))
	t.Cleanup(func() { util.Logger = orig })
	return f
}

const (
	pipelinesUse = `{"flowId":"flow-1","count":2,"pipelineIds":["p1","p2"]}`
	noPipelines  = ``
)

var releasesUse = lib.SmartServiceUsage{
	Releases:  3,
	Instances: 7,
	Readable:  []lib.SmartServiceRef{{Id: "r1", DesignId: "d1", Name: "heating"}},
}

func (f *deleteFixture) assertDeleted(t *testing.T, want ...string) {
	t.Helper()
	if strings.Join(f.db.deleted, ",") != strings.Join(want, ",") {
		t.Errorf("deleted %v, want %v", f.db.deleted, want)
	}
}

func TestDeleteFlowRefusesAFlowThatReleasesUse(t *testing.T) {
	f := newDeleteFixture(t, http.StatusNoContent, noPipelines, releasesUse)

	err := f.repo.DeleteFlow("flow-1", "user-a", "Bearer a", lib.DeleteOptions{})

	var inUse *lib.StillInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("err = %v, want a *StillInUseError", err)
	}
	if inUse.FlowUsage != nil || inUse.Smart == nil || inUse.Smart.Releases != 3 || inUse.Smart.Instances != 7 || len(inUse.Smart.Readable) != 1 {
		t.Errorf("usage = %+v / %+v", inUse.FlowUsage, inUse.Smart)
	}
	if f.smart.id != "flow-1" || f.smart.auth != "Bearer a" {
		t.Errorf("asked about %q with %q, want flow-1 with the caller's token", f.smart.id, f.smart.auth)
	}
	f.assertDeleted(t)
}

func TestDeleteFlowRefusalNamesPipelinesAndReleases(t *testing.T) {
	f := newDeleteFixture(t, http.StatusOK, pipelinesUse, releasesUse)

	err := f.repo.DeleteFlow("flow-1", "user-a", "Bearer a", lib.DeleteOptions{})

	var inUse *lib.StillInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("err = %v, want a *StillInUseError", err)
	}
	if got := lib.PipelineCount(inUse.FlowUsage); got != 2 {
		t.Errorf("pipelines = %d, want 2", got)
	}
	if inUse.Smart == nil || inUse.Smart.Releases != 3 {
		t.Errorf("smart = %+v, want the releases", inUse.Smart)
	}
	f.assertDeleted(t)
}

// The refusal for pipelines existed before; it must hold without any release.
func TestDeleteFlowStillRefusesAFlowThatOnlyPipelinesUse(t *testing.T) {
	f := newDeleteFixture(t, http.StatusOK, pipelinesUse, lib.SmartServiceUsage{Readable: []lib.SmartServiceRef{}})

	err := f.repo.DeleteFlow("flow-1", "user-a", "Bearer a", lib.DeleteOptions{})

	var inUse *lib.StillInUseError
	if !errors.As(err, &inUse) || inUse.Smart != nil || lib.PipelineCount(inUse.FlowUsage) != 2 {
		t.Fatalf("err = %v, want a *StillInUseError for two pipelines only", err)
	}
	f.assertDeleted(t)
}

// Only releases count; instances without a release do not hold the flow.
func TestDeleteFlowDeletesAnUnusedFlow(t *testing.T) {
	f := newDeleteFixture(t, http.StatusNoContent, noPipelines, lib.SmartServiceUsage{Instances: 4, Readable: []lib.SmartServiceRef{}})

	err := f.repo.DeleteFlow("flow-1", "user-a", "Bearer a", lib.DeleteOptions{})

	if err != nil {
		t.Fatal(err)
	}
	f.assertDeleted(t, "flow-1")
	if f.logs.Len() != 0 {
		t.Errorf("logged %q for an ordinary delete", f.logs.String())
	}
}

func TestDeleteFlowWithoutAnAnswerOfTheSmartServiceRepositoryDeletesNothing(t *testing.T) {
	for _, force := range []bool{false, true} {
		f := newDeleteFixture(t, http.StatusNoContent, noPipelines, lib.SmartServiceUsage{})
		f.smart.err = errors.New("smart-service-repository answered 500")

		err := f.repo.DeleteFlow("flow-1", "user-a", "Bearer a", lib.DeleteOptions{Force: force})

		var unavailable *lib.UsageUnavailableError
		if !errors.As(err, &unavailable) {
			t.Errorf("force=%v: err = %v, want a *UsageUnavailableError", force, err)
		}
		f.assertDeleted(t)
	}
}

func TestDeleteFlowWithoutAnAnswerOfThePipelineRegistryDeletesNothing(t *testing.T) {
	for _, force := range []bool{false, true} {
		f := newDeleteFixture(t, http.StatusInternalServerError, `oops`, lib.SmartServiceUsage{})

		err := f.repo.DeleteFlow("flow-1", "user-a", "Bearer a", lib.DeleteOptions{Force: force})

		var external *lib.ExternalResourceError
		if !errors.As(err, &external) {
			t.Errorf("force=%v: err = %v, want a *ExternalResourceError", force, err)
		}
		f.assertDeleted(t)
	}
}

func TestDeleteFlowForceSkipsBothRefusalsAndLogs(t *testing.T) {
	f := newDeleteFixture(t, http.StatusOK, pipelinesUse, releasesUse)

	err := f.repo.DeleteFlow("flow-1", "admin-a", "Bearer a", lib.DeleteOptions{Force: true})

	if err != nil {
		t.Fatal(err)
	}
	f.assertDeleted(t, "flow-1")
	logged := f.logs.String()
	for _, want := range []string{"level=WARN", "flow_id=flow-1", "user_id=admin-a", "pipelines=2", "releases=3", "instances=7"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log %q lacks %q", logged, want)
		}
	}
}

func TestNewRefusesAnEmptySmartServiceRepositoryUrl(t *testing.T) {
	_, err := New(srv_info_hdl.Handler{}, nil, nil, pipelinesClient.Client{}, "")
	if err == nil || !strings.Contains(err.Error(), "smart-service-repository") {
		t.Errorf("err = %v, want a refusal naming the smart-service-repository", err)
	}
}
