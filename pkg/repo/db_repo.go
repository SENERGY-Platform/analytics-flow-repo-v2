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
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/models"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	permV2Model "github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"strconv"
	"strings"
	"time"
)

type FlowRepository interface {
	InsertFlow(flow models.Flow) (err error)
	UpdateFlow(id string, flow models.Flow, userId string) (err error)
	DeleteFlow(id string, userId string, admin bool) (err error)
	All(userId string, admin bool, args map[string][]string) (response models.FlowsResponse, err error)
	FindFlow(id string, userId string) (flow models.Flow, err error)
}

type MongoRepo struct {
	perm permV2Client.Client
}

func NewMongoRepo(perm permV2Client.Client) *MongoRepo {
	_, err, _ := perm.SetTopic(permV2Client.InternalAdminToken, permV2Client.Topic{
		Id: PermV2InstanceTopic,
		DefaultPermissions: permV2Client.ResourcePermissions{
			RolePermissions: map[string]permV2Model.PermissionsMap{
				"admin": {
					Read:         true,
					Write:        true,
					Execute:      true,
					Administrate: true,
				},
			},
		},
	})
	if err != nil {
		return nil
	}
	return &MongoRepo{perm: perm}
}

func (r *MongoRepo) InsertFlow(flow models.Flow) (err error) {
	flow.DateCreated = time.Now()
	flow.DateUpdated = time.Now()
	_, err = Mongo().InsertOne(CTX, flow)
	if err != nil {
		return err
	}
	return
}

func (r *MongoRepo) UpdateFlow(id string, flow models.Flow, userId string) (err error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}
	flow.DateUpdated = time.Now()
	_, err = Mongo().ReplaceOne(CTX, bson.M{"_id": objID,
		"$or": []interface{}{
			bson.M{"userId": userId},
			bson.M{"share.write": true},
		}}, flow)
	return
}

func (r *MongoRepo) DeleteFlow(id string, userId string, admin bool) (err error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}
	req := bson.M{"_id": objID, "userId": userId}
	if admin {
		req = bson.M{"_id": objID}
	}
	res := Mongo().FindOneAndDelete(CTX, req)
	return res.Err()
}

func (r *MongoRepo) All(userId string, admin bool, args map[string][]string) (response models.FlowsResponse, err error) {
	opt := options.Find()
	for arg, value := range args {
		if arg == "limit" {
			limit, _ := strconv.ParseInt(value[0], 10, 64)
			opt.SetLimit(limit)
		}
		if arg == "offset" {
			skip, _ := strconv.ParseInt(value[0], 10, 64)
			opt.SetSkip(skip)
		}
		if arg == "order" {
			ord := strings.Split(value[0], ":")
			order := 1
			if ord[1] == "desc" {
				order = -1
			}
			opt.SetSort(bson.M{ord[0]: int64(order)})
		}
	}
	var cur *mongo.Cursor
	req := bson.M{"userId": userId}
	if val, ok := args["search"]; ok {
		req = bson.M{"userId": userId, "name": bson.M{"$regex": val[0]}}
	}
	if admin {
		req = bson.M{}
	}
	cur, err = Mongo().Find(CTX, req, opt)
	if err != nil {
		log.Println(err)
		return
	}
	req = bson.M{"userId": userId}
	if admin {
		req = bson.M{}
	}
	response.Total, err = Mongo().CountDocuments(CTX, req)
	if err != nil {
		log.Println(err)
		return
	}
	response.Flows = make([]models.Flow, 0)
	for cur.Next(CTX) {
		// create a value into which the single document can be decoded
		var elem models.Flow
		err = cur.Decode(&elem)
		if err != nil {
			return
		}
		response.Flows = append(response.Flows, elem)
	}
	return
}

func (r *MongoRepo) FindFlow(id string, userId string) (flow models.Flow, err error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}
	err = Mongo().FindOne(CTX, bson.M{"_id": objID,
		"$or": []interface{}{
			bson.M{"userId": userId},
			bson.M{"share.read": true},
		}}).Decode(&flow)
	if err != nil {
		log.Println(err)
		return
	}
	return
}
