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
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ownerPerm keeps permissions in memory and hands out and stores copies, as the client does through JSON.
type ownerPerm struct {
	permV2Client.Client
	resources map[string]permV2Client.ResourcePermissions
	gets      int
	sets      int
	failSet   error
}

func copyPermissions(in permV2Client.ResourcePermissions) (out permV2Client.ResourcePermissions) {
	b, err := json.Marshal(in)
	if err == nil {
		err = json.Unmarshal(b, &out)
	}
	if err != nil {
		panic(err)
	}
	return out
}

func (p *ownerPerm) GetResource(token string, topic string, id string) (permV2Client.Resource, error, int) {
	p.gets++
	if token != permV2Client.InternalAdminToken || topic != PermV2InstanceTopic {
		return permV2Client.Resource{}, errors.New("unexpected token or topic"), http.StatusBadRequest
	}
	r, ok := p.resources[id]
	if !ok {
		return permV2Client.Resource{}, errors.New("unexpected statuscode 404: not found"), http.StatusNotFound
	}
	return permV2Client.Resource{Id: id, TopicId: topic, ResourcePermissions: copyPermissions(r)}, nil, http.StatusOK
}

func (p *ownerPerm) SetPermission(token string, topic string, id string, permissions permV2Client.ResourcePermissions) (permV2Client.ResourcePermissions, error, int) {
	p.sets++
	if p.failSet != nil {
		return permissions, p.failSet, http.StatusInternalServerError
	}
	if token != permV2Client.InternalAdminToken || topic != PermV2InstanceTopic {
		return permissions, errors.New("unexpected token or topic"), http.StatusBadRequest
	}
	p.resources[id] = copyPermissions(permissions)
	return permissions, nil, http.StatusOK
}

func storedFlowPermissions() permV2Client.ResourcePermissions {
	return permV2Client.ResourcePermissions{
		UserPermissions: map[string]permV2Client.PermissionsMap{
			"user-a": fullRights,
			"user-b": {Read: true},
		},
		GroupPermissions: map[string]permV2Client.PermissionsMap{"/team": {Read: true, Execute: true}},
		RolePermissions:  map[string]permV2Client.PermissionsMap{"admin": fullRights},
	}
}

var ownerTestUpdated = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

// rawFlow reads the document as stored, so the legacy userid key is visible.
func rawFlow(t *testing.T, id primitive.ObjectID) bson.M {
	t.Helper()
	var doc bson.M
	if err := Mongo().FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// insertOwnedFlow stores a flow of user-a with name, image and both timestamps set; legacy stores
// the owner under userid instead of userId.
func insertOwnedFlow(t *testing.T, legacy bool) (primitive.ObjectID, *ownerPerm, *MongoRepo) {
	t.Helper()
	useTestMongo(t)
	id := primitive.NewObjectID()
	doc := bson.M{
		"_id": id, "name": "heating", "image": "<svg/>",
		"model":       bson.M{"cells": bson.A{bson.M{"type": "senergy.NodeElement", "id": "n1"}}},
		"dateCreated": ownerTestUpdated.Add(-time.Hour), "dateUpdated": ownerTestUpdated,
	}
	if legacy {
		doc["userid"] = "user-a"
	} else {
		doc["userId"] = "user-a"
	}
	if _, err := Mongo().InsertOne(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	perm := &ownerPerm{resources: map[string]permV2Client.ResourcePermissions{id.Hex(): storedFlowPermissions()}}
	return id, perm, &MongoRepo{perm: perm}
}

func assertOnlyOwnerChanged(t *testing.T, before, after bson.M, owner string) {
	t.Helper()
	want := bson.M{}
	for k, v := range before {
		if k != "userid" {
			want[k] = v
		}
	}
	want["userId"] = owner
	if !reflect.DeepEqual(after, want) {
		t.Errorf("document = %v\nwant only the owner changed: %v", after, want)
	}
}

func TestSetFlowOwnerMovesTheFlowAndKeepsEveryTimestamp(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		id, perm, r := insertOwnedFlow(t, legacy)
		before := rawFlow(t, id)

		flow, err := r.SetFlowOwner(id.Hex(), "user-c")

		if err != nil {
			t.Fatalf("legacy %v: %v", legacy, err)
		}
		after := rawFlow(t, id)
		assertOnlyOwnerChanged(t, before, after, "user-c")
		if _, left := after["userid"]; left {
			t.Errorf("legacy %v: the legacy userid key is still stored", legacy)
		}
		if got := storedFlow(t, id); !got.DateUpdated.Equal(ownerTestUpdated) || got.UserId != "user-c" {
			t.Errorf("legacy %v: dateUpdated = %v, owner %q", legacy, got.DateUpdated, got.UserId)
		}
		if !flow.DateUpdated.Equal(ownerTestUpdated) || flow.UserId != "user-c" || flow.Name != "heating" || flow.Id == nil || *flow.Id != id {
			t.Errorf("legacy %v: answered %+v", legacy, flow)
		}
		want := storedFlowPermissions()
		want.UserPermissions["user-c"] = fullRights
		if !reflect.DeepEqual(perm.resources[id.Hex()], want) {
			t.Errorf("legacy %v: permissions = %+v\nwant %+v", legacy, perm.resources[id.Hex()], want)
		}
	}
}

func TestSetFlowOwnerRaisesPartialRightsOfTheNewOwner(t *testing.T) {
	id, perm, r := insertOwnedFlow(t, false)

	if _, err := r.SetFlowOwner(id.Hex(), "user-b"); err != nil {
		t.Fatal(err)
	}

	want := storedFlowPermissions()
	want.UserPermissions["user-b"] = fullRights
	if !reflect.DeepEqual(perm.resources[id.Hex()], want) {
		t.Errorf("permissions = %+v\nwant %+v", perm.resources[id.Hex()], want)
	}
}

func TestSetFlowOwnerOfAnUnknownIdIsNotFoundAndAsksNobody(t *testing.T) {
	_, perm, r := insertOwnedFlow(t, false)

	for _, id := range []string{primitive.NewObjectID().Hex(), "no-object-id"} {
		_, err := r.SetFlowOwner(id, "user-c")

		var notFound *lib.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("id %q: got %v (%T), want a NotFoundError", id, err, err)
		}
	}
	if perm.gets != 0 || perm.sets != 0 {
		t.Errorf("asked permissions-v2 %d times and wrote %d times", perm.gets, perm.sets)
	}
}

func TestSetFlowOwnerWithoutPermissionsEntryChangesNothing(t *testing.T) {
	id, perm, r := insertOwnedFlow(t, true)
	delete(perm.resources, id.Hex())
	before := rawFlow(t, id)

	_, err := r.SetFlowOwner(id.Hex(), "user-c")

	var notFound *lib.NotFoundError
	if err == nil || errors.As(err, &notFound) {
		t.Fatalf("err = %v, want a failure that answers 500", err)
	}
	if perm.sets != 0 || !reflect.DeepEqual(rawFlow(t, id), before) {
		t.Errorf("wrote permissions %d times, document = %v", perm.sets, rawFlow(t, id))
	}
}

func TestSetFlowOwnerKeepsTheDocumentWhenPermissionsFail(t *testing.T) {
	id, perm, r := insertOwnedFlow(t, false)
	perm.failSet = errors.New("permissions-v2 down")
	before := rawFlow(t, id)

	if _, err := r.SetFlowOwner(id.Hex(), "user-c"); err == nil {
		t.Fatal("want an error")
	}

	if !reflect.DeepEqual(rawFlow(t, id), before) {
		t.Errorf("document = %v, want it untouched", rawFlow(t, id))
	}
}

// Rights are written before the document, so a move that stopped in between is "rights written,
// document not"; the repeat must finish it.
func TestSetFlowOwnerRepeatCompletesAHalfDoneMove(t *testing.T) {
	id, perm, r := insertOwnedFlow(t, true)
	before := rawFlow(t, id)
	half := storedFlowPermissions()
	half.UserPermissions["user-c"] = fullRights
	perm.resources[id.Hex()] = half

	flow, err := r.SetFlowOwner(id.Hex(), "user-c")

	if err != nil {
		t.Fatal(err)
	}
	if perm.sets != 0 {
		t.Errorf("wrote the permissions again %d times", perm.sets)
	}
	assertOnlyOwnerChanged(t, before, rawFlow(t, id), "user-c")
	if flow.UserId != "user-c" || !flow.DateUpdated.Equal(ownerTestUpdated) {
		t.Errorf("answered %+v", flow)
	}
}

func TestSetFlowOwnerTwiceChangesNothingTheSecondTime(t *testing.T) {
	id, perm, r := insertOwnedFlow(t, false)
	first, err := r.SetFlowOwner(id.Hex(), "user-c")
	if err != nil {
		t.Fatal(err)
	}
	afterFirst, permsAfterFirst := rawFlow(t, id), copyPermissions(perm.resources[id.Hex()])
	perm.sets = 0

	second, err := r.SetFlowOwner(id.Hex(), "user-c")

	if err != nil {
		t.Fatal(err)
	}
	if perm.sets != 0 || !reflect.DeepEqual(rawFlow(t, id), afterFirst) || !reflect.DeepEqual(perm.resources[id.Hex()], permsAfterFirst) || !reflect.DeepEqual(first, second) {
		t.Errorf("repeat changed something: sets %d, %v", perm.sets, rawFlow(t, id))
	}
}

// ownerDb fails the test when the service reaches the database for an invalid owner.
type ownerDb struct {
	FlowRepository
	calls int
}

func (o *ownerDb) SetFlowOwner(string, string) (lib.Flow, error) {
	o.calls++
	return lib.Flow{}, nil
}

func TestRepoSetFlowOwnerRejectsAnOwnerThatIsEmptyOrPadded(t *testing.T) {
	for _, owner := range []string{"", " ", " user-c", "user-c\n"} {
		db := &ownerDb{}
		r := &Repo{dbRepo: db}

		_, err := r.SetFlowOwner("flow-1", owner)

		var input *lib.InputError
		if !errors.As(err, &input) || db.calls != 0 {
			t.Errorf("owner %q: err = %v, database calls = %d, want an InputError and none", owner, err, db.calls)
		}
	}
	db := &ownerDb{}
	if _, err := (&Repo{dbRepo: db}).SetFlowOwner("flow-1", "user-c"); err != nil || db.calls != 1 {
		t.Errorf("valid owner: err = %v, database calls = %d", err, db.calls)
	}
}
