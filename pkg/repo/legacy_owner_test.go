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
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	pipelinesClient "github.com/SENERGY-Platform/analytics-pipeline/client"
	srv_info_hdl "github.com/SENERGY-Platform/go-service-base/srv-info-hdl"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	logs := &bytes.Buffer{}
	orig := util.Logger
	util.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(func() { util.Logger = orig })
	return logs
}

// testPermissions is the in-memory permissions-v2 that main uses for "mock"; it validates like the service.
func testPermissions(t *testing.T) permV2Client.Client {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	perm, err := permV2Client.NewTestClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return perm
}

func TestOwnerPermissionsReplacesTheEmptyUserWithOwner(t *testing.T) {
	permissions := storedFlowPermissions()
	permissions.UserPermissions[""] = fullRights

	ok := ownerPermissions(lib.Flow{UserId: "user-c"}, permissions)

	want := storedFlowPermissions()
	want.UserPermissions["user-c"] = fullRights
	if !ok || !reflect.DeepEqual(permissions, want) {
		t.Errorf("ok = %v, permissions = %+v\nwant %+v", ok, permissions, want)
	}
}

func TestOwnerPermissionsWithoutOwnerGrantsNobody(t *testing.T) {
	t.Run("another user administrates", func(t *testing.T) {
		permissions := storedFlowPermissions()
		permissions.UserPermissions[""] = fullRights

		ok := ownerPermissions(lib.Flow{}, permissions)

		if !ok || !reflect.DeepEqual(permissions, storedFlowPermissions()) {
			t.Errorf("ok = %v, permissions = %+v, want the stored ones without the empty user", ok, permissions)
		}
	})
	t.Run("only the empty user administrates", func(t *testing.T) {
		permissions := permV2Client.ResourcePermissions{UserPermissions: map[string]permV2Client.PermissionsMap{"": fullRights, "user-b": {Read: true}}}

		ok := ownerPermissions(lib.Flow{}, permissions)

		if ok {
			t.Error("reported permissions without an administrating user as storable")
		}
		if _, left := permissions.UserPermissions[""]; left {
			t.Error("the empty user keeps its rights")
		}
	})
}

// The repair skips an ownerless flow whose permissions would have no administrating user, because
// permissions-v2 refuses them and the error would stop the service at startup.
func TestPermissionsV2RefusesAnEntryWithoutAdministratingUser(t *testing.T) {
	perm := testPermissions(t)
	if NewMongoRepo(perm) == nil {
		t.Fatal("could not set the topic")
	}
	_, err, _ := perm.SetPermission(permV2Client.InternalAdminToken, PermV2InstanceTopic, primitive.NewObjectID().Hex(), permV2Client.ResourcePermissions{
		UserPermissions:  map[string]permV2Client.PermissionsMap{"user-b": {Read: true}},
		GroupPermissions: map[string]permV2Client.PermissionsMap{},
		RolePermissions:  map[string]permV2Client.PermissionsMap{"admin": fullRights},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid permissions") {
		t.Errorf("err = %v, want permissions-v2 to refuse permissions without an administrating user", err)
	}
}

var legacyUpdated = time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)

// insertRawFlow stores a flow with the given owner keys and returns the document as read back.
func insertRawFlow(t *testing.T, name string, owner bson.M) (primitive.ObjectID, bson.M) {
	t.Helper()
	id := primitive.NewObjectID()
	doc := bson.M{
		"_id": id, "name": name,
		"model":       bson.M{"cells": bson.A{bson.M{"type": "senergy.NodeElement", "id": "n1"}}},
		"dateCreated": legacyUpdated.Add(-time.Hour), "dateUpdated": legacyUpdated,
	}
	for k, v := range owner {
		doc[k] = v
	}
	if _, err := Mongo().InsertOne(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	return id, rawFlow(t, id)
}

// missing marks an owner key that must not be stored.
const missing = "<missing>"

func TestMigrateLegacyOwnersMovesOnlyOwnerKey(t *testing.T) {
	useTestMongo(t)
	logs := captureLogs(t)
	cases := []struct {
		name      string
		owner     bson.M
		want      any // userId after the migration; missing for none
		untouched bool
	}{
		{"legacy only", bson.M{"userid": "user-a"}, "user-a", false},
		{"empty userId", bson.M{"userid": "user-a", "userId": ""}, "user-a", false},
		{"null userId", bson.M{"userid": "user-a", "userId": nil}, "user-a", false},
		{"different owners", bson.M{"userid": "user-a", "userId": "user-b"}, "user-b", false},
		{"same owner", bson.M{"userid": "user-a", "userId": "user-a"}, "user-a", false},
		{"null legacy", bson.M{"userid": nil}, missing, false},
		{"empty legacy", bson.M{"userid": "", "userId": "user-b"}, "user-b", false},
		{"both empty", bson.M{"userid": "", "userId": ""}, "", false},
		{"legacy of another type", bson.M{"userid": int32(42)}, nil, true},
		{"no legacy", bson.M{"userId": "user-c"}, nil, true},
	}
	ids := map[string]primitive.ObjectID{}
	before := map[string]bson.M{}
	for _, c := range cases {
		ids[c.name], before[c.name] = insertRawFlow(t, c.name, c.owner)
	}

	matched, modified, err := migrateLegacyOwners(context.Background(), Mongo())

	if err != nil {
		t.Fatal(err)
	}
	if matched != 8 || modified != 8 {
		t.Errorf("matched %d, modified %d, want 8 each", matched, modified)
	}
	after := map[string]bson.M{}
	for _, c := range cases {
		after[c.name] = rawFlow(t, ids[c.name])
		want := bson.M{}
		for k, v := range before[c.name] {
			if c.untouched || (k != "userid" && k != "userId") {
				want[k] = v
			}
		}
		if !c.untouched && c.want != missing {
			want["userId"] = c.want
		}
		if !reflect.DeepEqual(after[c.name], want) {
			t.Errorf("%s: document = %v\nwant %v", c.name, after[c.name], want)
		}
	}
	if got := storedFlow(t, ids["legacy only"]); got.UserId != "user-a" || !got.DateUpdated.Equal(legacyUpdated) {
		t.Errorf("legacy flow decodes with owner %q, dateUpdated %v", got.UserId, got.DateUpdated)
	}

	out := logs.String()
	for _, c := range cases {
		conflict := strings.Contains(out, "two different owners") && strings.Contains(out, "flow_id="+ids[c.name].Hex())
		if conflict != (c.name == "different owners") {
			t.Errorf("%s: logged as conflict = %v", c.name, conflict)
		}
	}
	if !strings.Contains(out, "dropped_user_id=user-a") {
		t.Errorf("the dropped owner is not logged: %s", out)
	}
	if !strings.Contains(out, "neither a string nor null") || !strings.Contains(out, ids["legacy of another type"].Hex()) {
		t.Errorf("the flow left with userid is not logged: %s", out)
	}

	logs.Reset()
	matched, modified, err = migrateLegacyOwners(context.Background(), Mongo())

	if err != nil || matched != 0 || modified != 0 {
		t.Errorf("second run: matched %d, modified %d, err %v, want nothing", matched, modified, err)
	}
	for _, c := range cases {
		if doc := rawFlow(t, ids[c.name]); !reflect.DeepEqual(doc, after[c.name]) {
			t.Errorf("%s: second run changed the document to %v", c.name, doc)
		}
	}
	if strings.Contains(logs.String(), "level=INFO") || strings.Contains(logs.String(), "two different owners") {
		t.Errorf("second run logged more than debug and the leftover: %s", logs.String())
	}
}

func TestMongoAllListsALegacyFlowOnlyUnderUserId(t *testing.T) {
	useTestMongo(t)
	captureLogs(t)
	insertRawFlow(t, "legacy", bson.M{"userid": "user-a"})
	r := &MongoRepo{perm: readablePerm{}}
	names := func() []string {
		resp, err := r.All("user-a", false, map[string][]string{}, "")
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, f := range resp.Flows {
			names = append(names, f.Name)
		}
		return names
	}

	if got := names(); len(got) != 0 {
		t.Errorf("before the migration listed %v: the listing still matches userid", got)
	}
	if _, _, err := migrateLegacyOwners(context.Background(), Mongo()); err != nil {
		t.Fatal(err)
	}
	if got := names(); !slices.Equal(got, []string{"legacy"}) {
		t.Errorf("after the migration listed %v, want the legacy flow", got)
	}
}

func setFlowPermissions(t *testing.T, perm permV2Client.Client, id primitive.ObjectID, users map[string]permV2Client.PermissionsMap) {
	t.Helper()
	_, err, _ := perm.SetPermission(permV2Client.InternalAdminToken, PermV2InstanceTopic, id.Hex(), permV2Client.ResourcePermissions{
		UserPermissions:  users,
		GroupPermissions: map[string]permV2Client.PermissionsMap{},
		RolePermissions:  map[string]permV2Client.PermissionsMap{},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func userPermissions(t *testing.T, perm permV2Client.Client, id primitive.ObjectID) map[string]permV2Client.PermissionsMap {
	t.Helper()
	resource, err, _ := perm.GetResource(permV2Client.InternalAdminToken, PermV2InstanceTopic, id.Hex())
	if err != nil {
		t.Fatal(err)
	}
	return resource.UserPermissions
}

func TestValidateFlowPermissionsGrantsNothingToTheEmptyUser(t *testing.T) {
	useTestMongo(t)
	logs := captureLogs(t)
	perm := testPermissions(t)
	r := NewMongoRepo(perm)
	owned, _ := insertRawFlow(t, "owned", bson.M{"userId": "user-a"})
	ownerless, _ := insertRawFlow(t, "ownerless", bson.M{})
	onlyEmpty, _ := insertRawFlow(t, "only the empty user administrates", bson.M{"userId": ""})
	unregistered, _ := insertRawFlow(t, "ownerless without permissions", bson.M{})
	setFlowPermissions(t, perm, owned, map[string]permV2Client.PermissionsMap{"": fullRights, "user-b": {Read: true}})
	setFlowPermissions(t, perm, ownerless, map[string]permV2Client.PermissionsMap{"": fullRights, "user-b": fullRights})
	setFlowPermissions(t, perm, onlyEmpty, map[string]permV2Client.PermissionsMap{"": fullRights})

	if err := r.validateFlowPermissions(); err != nil {
		t.Fatalf("the repair failed and would stop the service: %v", err)
	}

	if got, want := userPermissions(t, perm, owned), map[string]permV2Client.PermissionsMap{"user-a": fullRights, "user-b": {Read: true}}; !reflect.DeepEqual(got, want) {
		t.Errorf("owned: %v, want %v", got, want)
	}
	if got, want := userPermissions(t, perm, ownerless), map[string]permV2Client.PermissionsMap{"user-b": fullRights}; !reflect.DeepEqual(got, want) {
		t.Errorf("ownerless: %v, want %v", got, want)
	}
	// permissions-v2 refuses an entry without an administrating user, so the stored one stays.
	if got, want := userPermissions(t, perm, onlyEmpty), map[string]permV2Client.PermissionsMap{"": fullRights}; !reflect.DeepEqual(got, want) {
		t.Errorf("only the empty user: %v, want %v", got, want)
	}
	if _, err, _ := perm.GetResource(permV2Client.InternalAdminToken, PermV2InstanceTopic, unregistered.Hex()); err == nil {
		t.Error("created a permissions entry for an ownerless flow")
	}
	for _, id := range []primitive.ObjectID{ownerless, onlyEmpty, unregistered} {
		if !strings.Contains(logs.String(), `flow has no owner, granting no default rights" flow_id=`+id.Hex()) {
			t.Errorf("no warning for ownerless flow %s: %s", id.Hex(), logs.String())
		}
	}
}

func TestNewMigratesOwnersBeforeRepairingPermissions(t *testing.T) {
	useTestMongo(t)
	captureLogs(t)
	perm := testPermissions(t)
	if NewMongoRepo(perm) == nil {
		t.Fatal("could not set the topic")
	}
	id, _ := insertRawFlow(t, "legacy", bson.M{"userid": "user-a"})
	// What the repair granted before the migration existed.
	setFlowPermissions(t, perm, id, map[string]permV2Client.PermissionsMap{"": fullRights})

	if _, err := New(srv_info_hdl.Handler{}, perm, nil, pipelinesClient.Client{}, "http://smart-service-repository"); err != nil {
		t.Fatal(err)
	}

	if got, want := userPermissions(t, perm, id), map[string]permV2Client.PermissionsMap{"user-a": fullRights}; !reflect.DeepEqual(got, want) {
		t.Errorf("permissions = %v, want only the legacy owner", got)
	}
	if doc := rawFlow(t, id); doc["userId"] != "user-a" || doc["userid"] != nil {
		t.Errorf("document = %v", doc)
	}
}
