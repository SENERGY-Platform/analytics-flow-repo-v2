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
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// readablePerm lets every caller read the listed flow ids.
type readablePerm struct {
	permV2Client.Client
	ids []string
}

func (p readablePerm) ListAccessibleResourceIds(string, string, permV2Client.ListOptions, ...permV2Client.Permission) ([]string, error, int) {
	return p.ids, nil, 200
}

// useTestMongo points the package at a scratch database on MONGO_TEST_URL and skips without it.
func useTestMongo(t *testing.T) {
	t.Helper()
	url := os.Getenv("MONGO_TEST_URL")
	if url == "" {
		t.Skip("MONGO_TEST_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(url))
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}
	oldDB, oldDatabase := DB, database
	DB, database = client, "flow_usage_test_"+primitive.NewObjectID().Hex()
	t.Cleanup(func() {
		_ = client.Database(database).Drop(context.Background())
		_ = client.Disconnect(context.Background())
		DB, database = oldDB, oldDatabase
	})
}

func insertFlow(t *testing.T, flow lib.Flow) {
	t.Helper()
	if _, err := Mongo().InsertOne(context.Background(), flow); err != nil {
		t.Fatal(err)
	}
}

func TestMongoStoresThePinAsOperatorversion(t *testing.T) {
	useTestMongo(t)
	flow := pinnedFlow("pinned", "user-a", [2]string{"op-1", "v1.0.0"}, [2]string{"op-1", ""})
	insertFlow(t, flow)

	var raw bson.M
	if err := Mongo().FindOne(context.Background(), bson.M{"_id": flow.Id}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	cells := raw["model"].(bson.M)["cells"].(bson.A)
	pinned, unpinned := cells[0].(bson.M), cells[1].(bson.M)
	if pinned["operatorversion"] != "v1.0.0" {
		t.Errorf("stored pin = %v in %v, want operatorversion v1.0.0", pinned["operatorversion"], pinned)
	}
	// Cell has no bson omitempty, so an absent pin is stored as null.
	if unpinned["operatorversion"] != nil {
		t.Errorf("an unpinned node stores a pin: %v", unpinned["operatorversion"])
	}
}

func TestMongoUsageOfOperatorVersion(t *testing.T) {
	useTestMongo(t)
	pinned := pinnedFlow("pinned", "user-a", [2]string{"op-1", "v1.0.0"})
	otherUser := pinnedFlow("other user", "user-b", [2]string{"op-1", "v1.0.0"})
	otherLabel := pinnedFlow("other label", "user-a", [2]string{"op-1", "v2.0.0"})
	unpinned := pinnedFlow("unpinned", "user-a", [2]string{"op-1", ""})
	split := pinnedFlow("split", "user-a", [2]string{"op-1", ""}, [2]string{"op-2", "v1.0.0"})
	otherOperator := pinnedFlow("other operator", "user-a", [2]string{"op-2", "v1.0.0"})
	for _, f := range []lib.Flow{pinned, otherUser, otherLabel, unpinned, split, otherOperator} {
		insertFlow(t, f)
	}
	// A flow from before the key was spelled operatorid stores operatorId.
	legacy := primitive.NewObjectID()
	_, err := Mongo().InsertOne(context.Background(), bson.M{"_id": legacy, "name": "legacy", "model": bson.M{"cells": bson.A{
		bson.M{"type": "senergy.NodeElement", "operatorId": "op-1", "operatorversion": "v1.0.0"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	// The caller may read its own flows and the legacy one, not user-b's.
	perm := readablePerm{ids: []string{pinned.Id.Hex(), otherLabel.Id.Hex(), unpinned.Id.Hex(), split.Id.Hex(), otherOperator.Id.Hex(), legacy.Hex()}}
	r := &Repo{dbRepo: &MongoRepo{perm: perm}}

	usage, err := r.GetUsageOfOperator("op-1", "v1.0.0", "user-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if usage.Flows != 3 {
		t.Errorf("flows = %d, want 3: pinned, other user and legacy", usage.Flows)
	}
	var names []string
	for _, ref := range usage.Readable {
		names = append(names, ref.Name)
	}
	slices.Sort(names)
	if want := []string{"legacy", "pinned"}; !slices.Equal(names, want) {
		t.Errorf("readable = %v, want %v", names, want)
	}

	all, err := r.GetUsageOfOperator("op-1", "", "user-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if all.Flows != 6 {
		t.Errorf("without version: flows = %d, want 6", all.Flows)
	}
	if len(all.Readable) != 5 {
		t.Errorf("without version: readable = %d, want 5", len(all.Readable))
	}

	none, err := r.GetUsageOfOperator("op-1", "v3.0.0", "user-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if none.Flows != 0 || len(none.Readable) != 0 {
		t.Errorf("unpinned label: %+v, want none", none)
	}
}

func TestMongoFilterOperatorVersionWithoutOperatorIsIgnored(t *testing.T) {
	useTestMongo(t)
	insertFlow(t, pinnedFlow("pinned", "user-a", [2]string{"op-1", "v1.0.0"}))
	insertFlow(t, pinnedFlow("unpinned", "user-a", [2]string{"op-1", ""}))

	got, err := (&MongoRepo{}).All("", true, map[string][]string{"filter": {"operatorversion:v1.0.0"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 2 {
		t.Errorf("flows = %d, want both: the version only narrows the operator filter", len(got.Flows))
	}
}
