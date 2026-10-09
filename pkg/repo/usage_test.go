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
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fakeFlows keeps flows in memory. Like the real repository it counts the flows of all users in
// the mapping, and its listing returns only the flows its caller owns or was shared.
type fakeFlows struct {
	FlowRepository
	flows    []lib.Flow
	sharedTo map[string][]primitive.ObjectID
	mapErr   error
	allCalls int
	allUser  string
	allAuth  string
	allArgs  map[string][]string
}

func (f *fakeFlows) GetOperatorFlowMapping() ([]lib.OperatorFlowCount, error) {
	if f.mapErr != nil {
		return nil, f.mapErr
	}
	byOperator := map[string][]lib.FlowCount{}
	for _, flow := range f.flows {
		seen := map[string]int32{}
		for _, cell := range flow.Model.Cells {
			if cell.Type == "senergy.NodeElement" && cell.OperatorId != nil {
				seen[*cell.OperatorId]++
			}
		}
		for op, n := range seen {
			byOperator[op] = append(byOperator[op], lib.FlowCount{FlowID: flow.Id, Count: n})
		}
	}
	var result []lib.OperatorFlowCount
	for op, counts := range byOperator {
		result = append(result, lib.OperatorFlowCount{OperatorID: op, Flows: counts})
	}
	return result, nil
}

func (f *fakeFlows) All(userId string, _ bool, args map[string][]string, auth string) (lib.FlowsResponse, error) {
	f.allCalls++
	f.allUser, f.allAuth, f.allArgs = userId, auth, args
	var operators []string
	for _, raw := range args["filter"] {
		if op, ok := strings.CutPrefix(raw, "operator:"); ok {
			operators = append(operators, op)
		}
	}
	response := lib.FlowsResponse{Flows: []lib.Flow{}}
	for _, flow := range f.flows {
		if flow.UserId != userId && !slices.Contains(f.sharedTo[userId], *flow.Id) {
			continue
		}
		if len(operators) > 0 && !usesAny(flow, operators) {
			continue
		}
		response.Flows = append(response.Flows, flow)
	}
	response.Total = int64(len(response.Flows))
	return response, nil
}

func usesAny(flow lib.Flow, operators []string) bool {
	for _, cell := range flow.Model.Cells {
		if cell.Type == "senergy.NodeElement" && cell.OperatorId != nil && slices.Contains(operators, *cell.OperatorId) {
			return true
		}
	}
	return false
}

func testFlow(name, userId string, operatorIds ...string) lib.Flow {
	id := primitive.NewObjectID()
	flow := lib.Flow{Id: &id, Name: name, UserId: userId}
	for _, op := range operatorIds {
		flow.Model.Cells = append(flow.Model.Cells, lib.Cell{Type: "senergy.NodeElement", OperatorId: &op})
	}
	// A link is not a node; it must not count as a use.
	flow.Model.Cells = append(flow.Model.Cells, lib.Cell{Type: "link"})
	return flow
}

func TestGetUsageOfOperatorCountsFlowsOfAllUsers(t *testing.T) {
	a1 := testFlow("a1", "user-a", "op-1")
	a2 := testFlow("a2", "user-a", "op-1", "op-2")
	b1 := testFlow("b1", "user-b", "op-1")
	c1 := testFlow("c1", "user-c", "op-2")
	r := &Repo{dbRepo: &fakeFlows{flows: []lib.Flow{a1, a2, b1, c1}}}

	usage, err := r.GetUsageOfOperator("op-1", "user-a", "Bearer a")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Flows != 3 {
		t.Errorf("flows = %d, want 3", usage.Flows)
	}
}

func TestGetUsageOfOperatorCountsAFlowOnceWhenItUsesAnOperatorTwice(t *testing.T) {
	r := &Repo{dbRepo: &fakeFlows{flows: []lib.Flow{testFlow("twice", "user-a", "op-1", "op-1")}}}

	usage, err := r.GetUsageOfOperator("op-1", "user-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Flows != 1 || len(usage.Readable) != 1 {
		t.Errorf("usage = %+v, want one flow", usage)
	}
}

func TestGetUsageOfOperatorListsOnlyWhatTheCallerMayRead(t *testing.T) {
	a1 := testFlow("a1", "user-a", "op-1")
	b1 := testFlow("b1", "user-b", "op-1")
	b2 := testFlow("b2", "user-b", "op-1")
	c1 := testFlow("c1", "user-c", "op-1")
	db := &fakeFlows{
		flows:    []lib.Flow{a1, b1, b2, c1},
		sharedTo: map[string][]primitive.ObjectID{"user-a": {*b2.Id}},
	}
	r := &Repo{dbRepo: db}

	usage, err := r.GetUsageOfOperator("op-1", "user-a", "Bearer a")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Flows != 4 {
		t.Errorf("flows = %d, want 4: the count covers the flows the caller cannot read", usage.Flows)
	}
	want := []lib.FlowRef{{Id: a1.Id.Hex(), Name: "a1"}, {Id: b2.Id.Hex(), Name: "b2"}}
	if !slices.Equal(usage.Readable, want) {
		t.Errorf("readable = %v, want %v", usage.Readable, want)
	}
	if db.allUser != "user-a" || db.allAuth != "Bearer a" {
		t.Errorf("listing ran as %q with %q, want the caller", db.allUser, db.allAuth)
	}
}

func TestGetUsageOfOperatorForAnOperatorNoFlowUses(t *testing.T) {
	db := &fakeFlows{flows: []lib.Flow{testFlow("a1", "user-a", "op-1")}}
	r := &Repo{dbRepo: db}

	usage, err := r.GetUsageOfOperator("op-unused", "user-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Flows != 0 {
		t.Errorf("flows = %d, want 0", usage.Flows)
	}
	if usage.Readable == nil || len(usage.Readable) != 0 {
		t.Errorf("readable = %#v, want an empty list that marshals as []", usage.Readable)
	}
	if db.allCalls != 0 {
		t.Error("the flow listing ran for an operator nobody uses")
	}
}

func TestGetUsageOfOperatorReturnsAggregationError(t *testing.T) {
	boom := errors.New("mongo down")
	r := &Repo{dbRepo: &fakeFlows{mapErr: boom}}

	if _, err := r.GetUsageOfOperator("op-1", "user-a", ""); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
