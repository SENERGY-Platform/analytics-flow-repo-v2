/*
 * Copyright 2026 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// useUpdateTestMongo points the package at a scratch database on MONGO_TEST_URL and skips without it.
func useUpdateTestMongo(t *testing.T) {
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
	DB, database = client, "flow_update_test_"+primitive.NewObjectID().Hex()
	t.Cleanup(func() {
		_ = client.Database(database).Drop(context.Background())
		_ = client.Disconnect(context.Background())
		DB, database = oldDB, oldDatabase
	})
}

func insertUpdateTestFlow(t *testing.T, flow lib.Flow) {
	t.Helper()
	if _, err := Mongo().InsertOne(context.Background(), flow); err != nil {
		t.Fatal(err)
	}
}

// writablePerm grants every check.
type writablePerm struct {
	permV2Client.Client
}

func (writablePerm) CheckPermission(string, string, string, ...permV2Client.Permission) (bool, error, int) {
	return true, nil, 200
}

func storedFlow(t *testing.T, id primitive.ObjectID) lib.Flow {
	t.Helper()
	var flow lib.Flow
	if err := Mongo().FindOne(context.Background(), bson.M{"_id": id}).Decode(&flow); err != nil {
		t.Fatal(err)
	}
	return flow
}

func TestUpdateKeepsOwnerAndCreationTime(t *testing.T) {
	useUpdateTestMongo(t)
	repo := &MongoRepo{perm: writablePerm{}}
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("a body without them keeps the stored ones", func(t *testing.T) {
		id := primitive.NewObjectID()
		insertUpdateTestFlow(t, lib.Flow{Id: &id, Name: "flow", UserId: "owner", DateCreated: created})
		if err := repo.UpdateFlow(id.Hex(), lib.Flow{Name: "renamed"}, "someone", "token"); err != nil {
			t.Fatal(err)
		}
		flow := storedFlow(t, id)
		if flow.Name != "renamed" || flow.UserId != "owner" || !flow.DateCreated.Equal(created) {
			t.Errorf("stored name %q, owner %q, created %v", flow.Name, flow.UserId, flow.DateCreated)
		}
	})

	t.Run("a body cannot change them", func(t *testing.T) {
		id := primitive.NewObjectID()
		insertUpdateTestFlow(t, lib.Flow{Id: &id, Name: "flow", UserId: "owner", DateCreated: created})
		if err := repo.UpdateFlow(id.Hex(), lib.Flow{Name: "flow", UserId: "other", DateCreated: time.Now()}, "other", "token"); err != nil {
			t.Fatal(err)
		}
		flow := storedFlow(t, id)
		if flow.UserId != "owner" || !flow.DateCreated.Equal(created) {
			t.Errorf("stored owner %q, created %v", flow.UserId, flow.DateCreated)
		}
	})

	t.Run("a flow stored without them takes them from the body", func(t *testing.T) {
		id := primitive.NewObjectID()
		insertUpdateTestFlow(t, lib.Flow{Id: &id, Name: "legacy"})
		if err := repo.UpdateFlow(id.Hex(), lib.Flow{Name: "legacy", UserId: "owner", DateCreated: created}, "owner", "token"); err != nil {
			t.Fatal(err)
		}
		flow := storedFlow(t, id)
		if flow.UserId != "owner" || !flow.DateCreated.Equal(created) {
			t.Errorf("stored owner %q, created %v", flow.UserId, flow.DateCreated)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		err := repo.UpdateFlow(primitive.NewObjectID().Hex(), lib.Flow{Name: "x"}, "owner", "token")
		var notFound *lib.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("got %v (%T), want a NotFoundError", err, err)
		}
	})
}
