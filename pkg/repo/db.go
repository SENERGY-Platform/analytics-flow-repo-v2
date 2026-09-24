/*
 * Copyright 2025 InfAI (CC SES)
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
	"errors"
	"fmt"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var DB *mongo.Client
var CTX mongo.SessionContext
var database string

var (
	errEmptyDatabase   = errors.New("mongo database name must not be empty")
	errMissingPassword = errors.New("mongo password must not be empty when a mongo user is set")
)

func InitDB(cfg *config.Config) error {
	if err := validateMongoConfig(cfg); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions(cfg))
	if err != nil {
		return err
	}
	// Connect does not contact the server, and ping needs no authentication; listCollections on the
	// service's database fails at startup on unreachable hosts and on wrong or missing credentials.
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer checkCancel()
	listOpts := options.ListCollections().SetNameOnly(true).SetAuthorizedCollections(true)
	if _, err = client.Database(cfg.MongoDatabase).ListCollectionNames(checkCtx, bson.D{}, listOpts); err != nil {
		_ = client.Disconnect(context.Background())
		return fmt.Errorf("mongo startup check failed: %w", err)
	}
	DB = client
	database = cfg.MongoDatabase
	return nil
}

func validateMongoConfig(cfg *config.Config) error {
	if cfg.MongoDatabase == "" {
		return errEmptyDatabase
	}
	if cfg.MongoUser != "" && cfg.MongoPassword.Value() == "" {
		return errMissingPassword
	}
	return nil
}

// clientOptions applies the credentials after the URI so they take precedence over any given in MONGO_URL.
func clientOptions(cfg *config.Config) *options.ClientOptions {
	opts := options.Client().ApplyURI(cfg.MongoUrl)
	if cfg.MongoUser != "" {
		opts.SetAuth(options.Credential{
			Username:   cfg.MongoUser,
			Password:   cfg.MongoPassword.Value(),
			AuthSource: cfg.MongoAuthSource,
		})
	}
	return opts
}

func Mongo() *mongo.Collection {
	return flowsCollection(DB, database)
}

func flowsCollection(client *mongo.Client, database string) *mongo.Collection {
	return client.Database(database).Collection("flows")
}

func CloseDB() {
	err := DB.Disconnect(CTX)
	if err != nil {
		panic("failed to disconnect database: " + err.Error())
	}
}

func GetDB() *mongo.Client {
	return DB
}
