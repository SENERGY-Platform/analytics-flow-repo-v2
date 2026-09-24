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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/config"
	sb_config_types "github.com/SENERGY-Platform/go-service-base/config-hdl/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const replicaSetURL = "mongodb://mongo-0.mongo:27017,mongo-1.mongo:27017/?replicaSet=rs0"

func TestClientOptionsSetsAuthWhenUserGiven(t *testing.T) {
	opts := clientOptions(&config.Config{
		MongoUrl:        replicaSetURL,
		MongoUser:       "analytics-flow-repo",
		MongoPassword:   "s3cret",
		MongoAuthSource: "admin",
	})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if opts.Auth == nil {
		t.Fatal("expected credentials to be set")
	}
	if opts.Auth.Username != "analytics-flow-repo" {
		t.Errorf("username = %q", opts.Auth.Username)
	}
	if opts.Auth.Password != "s3cret" {
		t.Errorf("password does not match the configured secret")
	}
	if opts.Auth.AuthSource != "admin" {
		t.Errorf("auth source = %q", opts.Auth.AuthSource)
	}
	if opts.Auth.AuthMechanism != "" {
		t.Errorf("auth mechanism = %q, expected driver default", opts.Auth.AuthMechanism)
	}
}

func TestClientOptionsConfiguredAuthOverridesURI(t *testing.T) {
	opts := clientOptions(&config.Config{
		MongoUrl:        "mongodb://other:pw@localhost:27017/?authSource=other&authMechanism=SCRAM-SHA-1",
		MongoUser:       "analytics-flow-repo",
		MongoPassword:   "s3cret",
		MongoAuthSource: "admin",
	})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if opts.Auth == nil {
		t.Fatal("expected credentials to be set")
	}
	if opts.Auth.Username != "analytics-flow-repo" || opts.Auth.AuthSource != "admin" {
		t.Fatalf("configured credentials did not take precedence: user %q, auth source %q", opts.Auth.Username, opts.Auth.AuthSource)
	}
	if opts.Auth.AuthMechanism != "" {
		t.Errorf("auth mechanism %q from the uri was kept", opts.Auth.AuthMechanism)
	}
}

func TestClientOptionsNoAuthWithoutUser(t *testing.T) {
	opts := clientOptions(&config.Config{
		MongoUrl:        "mongodb://localhost:27017",
		MongoPassword:   "ignored-without-user",
		MongoAuthSource: "admin",
	})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if opts.Auth != nil {
		t.Fatalf("expected no credentials, got user %q", opts.Auth.Username)
	}
}

func TestClientOptionsPassesURIUnchanged(t *testing.T) {
	opts := clientOptions(&config.Config{MongoUrl: replicaSetURL})
	if err := opts.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if got := opts.GetURI(); got != replicaSetURL {
		t.Errorf("uri = %q, want %q", got, replicaSetURL)
	}
	if want := []string{"mongo-0.mongo:27017", "mongo-1.mongo:27017"}; !reflect.DeepEqual(opts.Hosts, want) {
		t.Errorf("hosts = %v, want %v", opts.Hosts, want)
	}
	if opts.ReplicaSet == nil || *opts.ReplicaSet != "rs0" {
		t.Errorf("replica set not taken from the uri: %v", opts.ReplicaSet)
	}
}

func TestClientOptionsDoesNotAddScheme(t *testing.T) {
	opts := clientOptions(&config.Config{MongoUrl: "localhost:27017"})
	if err := opts.Validate(); err == nil {
		t.Fatal("expected an error for a url without scheme")
	}
}

func TestFlowsCollectionUsesConfiguredDatabase(t *testing.T) {
	// mongo.Connect does not contact the server, so no running instance is needed.
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })

	coll := flowsCollection(client, "custom_db")
	if coll.Database().Name() != "custom_db" {
		t.Errorf("database = %q", coll.Database().Name())
	}
	if coll.Name() != "flows" {
		t.Errorf("collection = %q", coll.Name())
	}
}

func TestValidateMongoConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr error
	}{
		{"no auth", config.Config{MongoDatabase: "db"}, nil},
		{"user and password", config.Config{MongoDatabase: "db", MongoUser: "u", MongoPassword: "p"}, nil},
		{"password without user", config.Config{MongoDatabase: "db", MongoPassword: "p"}, nil},
		{"user without password", config.Config{MongoDatabase: "db", MongoUser: "u"}, errMissingPassword},
		{"empty database", config.Config{}, errEmptyDatabase},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateMongoConfig(&tt.cfg); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Port 1 refuses connections; the short selection timeout keeps a failing startup check fast.
const unreachableURL = "mongodb://127.0.0.1:1/?serverSelectionTimeoutMS=200"

// The startup check would fail as well, so the tests check for the specific validation error.
func TestInitDBRejectsEmptyDatabase(t *testing.T) {
	keepGlobals(t)
	if err := InitDB(&config.Config{MongoUrl: unreachableURL}); !errors.Is(err, errEmptyDatabase) {
		t.Fatalf("err = %v, want %v", err, errEmptyDatabase)
	}
}

func TestInitDBRejectsUserWithoutPassword(t *testing.T) {
	keepGlobals(t)
	err := InitDB(&config.Config{MongoUrl: unreachableURL, MongoDatabase: "db", MongoUser: "analytics-flow-repo"})
	if !errors.Is(err, errMissingPassword) {
		t.Fatalf("err = %v, want %v", err, errMissingPassword)
	}
}

func TestInitDBFailsWhenStartupCheckFails(t *testing.T) {
	const password = "pw-must-not-appear-7f3a"
	keepGlobals(t)
	DB, database = nil, ""
	err := InitDB(&config.Config{
		MongoUrl:        unreachableURL,
		MongoUser:       "analytics-flow-repo",
		MongoPassword:   password,
		MongoAuthSource: "admin",
		MongoDatabase:   "db",
	})
	if err == nil {
		t.Fatal("expected an error when the server is unreachable")
	}
	if !strings.HasPrefix(err.Error(), "mongo startup check failed: ") {
		t.Errorf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), password) {
		t.Error("error text contains the password")
	}
	if DB != nil || database != "" {
		t.Error("globals must stay unset after a failed startup check")
	}
}

// TestInitDBAuthenticates needs a throwaway server with access control; MONGO_AUTH_TEST_USER and
// MONGO_AUTH_TEST_PASSWORD are root credentials, used to create the test users.
func TestInitDBAuthenticates(t *testing.T) {
	url, rootUser, rootPassword := os.Getenv("MONGO_AUTH_TEST_URL"), os.Getenv("MONGO_AUTH_TEST_USER"), os.Getenv("MONGO_AUTH_TEST_PASSWORD")
	if testing.Short() || url == "" || rootUser == "" || rootPassword == "" {
		t.Skip("needs MONGO_AUTH_TEST_URL, MONGO_AUTH_TEST_USER and MONGO_AUTH_TEST_PASSWORD, not in -short")
	}
	ctx := context.Background()
	root, err := mongo.Connect(ctx, options.Client().ApplyURI(url).SetAuth(options.Credential{Username: rootUser, Password: rootPassword, AuthSource: "admin"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Disconnect(ctx) })

	suffix := randomHex(t)
	testDB, otherDB := "flow_auth_test_"+suffix, "flow_auth_other_"+suffix
	svcUser, svcPassword := "flow-test-"+suffix, randomHex(t)
	otherUser, otherPassword := "flow-other-"+suffix, randomHex(t)
	createUser(t, root, svcUser, svcPassword, testDB)
	createUser(t, root, otherUser, otherPassword, otherDB)

	cases := []struct {
		name, user, password string
		wantErr              bool
	}{
		{"correct credentials", svcUser, svcPassword, false},
		{"no credentials", "", "", true},
		{"user of another database", otherUser, otherPassword, true},
		{"wrong password", svcUser, svcPassword + "-wrong", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			keepGlobals(t)
			err := InitDB(&config.Config{
				MongoUrl:        url,
				MongoUser:       c.user,
				MongoPassword:   sb_config_types.Secret(c.password),
				MongoAuthSource: "admin",
				MongoDatabase:   testDB,
			})
			if err == nil {
				_ = DB.Disconnect(ctx)
			}
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if err == nil {
				return
			}
			if !strings.HasPrefix(err.Error(), "mongo startup check failed: ") {
				t.Errorf("unexpected error: %v", err)
			}
			for _, pw := range []string{svcPassword, otherPassword, rootPassword} {
				if strings.Contains(err.Error(), pw) {
					t.Error("error text contains a password")
				}
			}
		})
	}
}

func createUser(t *testing.T, root *mongo.Client, user, password, db string) {
	t.Helper()
	admin := root.Database("admin")
	cmd := bson.D{
		{Key: "createUser", Value: user},
		{Key: "pwd", Value: password},
		{Key: "roles", Value: bson.A{bson.D{{Key: "role", Value: "readWrite"}, {Key: "db", Value: db}}}},
	}
	if err := admin.RunCommand(context.Background(), cmd).Err(); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.RunCommand(context.Background(), bson.D{{Key: "dropUser", Value: user}}).Err()
		_ = root.Database(db).Drop(context.Background())
	})
}

func randomHex(t *testing.T) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func keepGlobals(t *testing.T) {
	prevDB, prevDatabase := DB, database
	t.Cleanup(func() { DB, database = prevDB, prevDatabase })
}
