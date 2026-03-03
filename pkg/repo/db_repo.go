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
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	permV2Model "github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FlowRepository interface {
	InsertFlow(flow lib.Flow) (err error)
	UpdateFlow(id string, flow lib.Flow, userId string, auth string) (err error)
	DeleteFlow(id string, userId string, admin bool, auth string) (err error)
	All(userId string, admin bool, args map[string][]string, auth string) (response lib.FlowsResponse, err error)
	FindFlow(id, userId, auth string) (flow lib.Flow, err error)
	GetOperatorFlowMapping() ([]lib.OperatorFlowCount, error)
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

func (r *MongoRepo) validateFlowPermissions() (err error) {
	util.Logger.Debug("validate flows permissions")
	resp, err := r.All("", true, map[string][]string{}, "")
	if err != nil {
		return
	}
	permResources, err, _ := r.perm.ListResourcesWithAdminPermission(permV2Client.InternalAdminToken, PermV2InstanceTopic, permV2Client.ListOptions{})
	if err != nil {
		return
	}
	permResourceMap := map[string]permV2Client.Resource{}
	for _, permResource := range permResources {
		permResourceMap[permResource.Id] = permResource
	}

	dbIds := []string{}
	for _, flow := range resp.Flows {
		permissions := permV2Client.ResourcePermissions{
			UserPermissions:  map[string]permV2Client.PermissionsMap{},
			GroupPermissions: map[string]permV2Client.PermissionsMap{},
			RolePermissions:  map[string]permV2Model.PermissionsMap{},
		}
		flowId := flow.Id.Hex()
		dbIds = append(dbIds, flowId)
		resource, ok := permResourceMap[flowId]
		if ok {
			permissions.UserPermissions = resource.ResourcePermissions.UserPermissions
			permissions.GroupPermissions = resource.GroupPermissions
			permissions.RolePermissions = resource.ResourcePermissions.RolePermissions
		}
		SetDefaultPermissions(flow, permissions)

		_, err, _ = r.perm.SetPermission(permV2Client.InternalAdminToken, PermV2InstanceTopic, flowId, permissions)
		if err != nil {
			return
		}
	}
	permResourceIds := maps.Keys(permResourceMap)

	for permResouceId := range permResourceIds {
		if !slices.Contains(dbIds, permResouceId) {
			err, _ = r.perm.RemoveResource(permV2Client.InternalAdminToken, PermV2InstanceTopic, permResouceId)
			if err != nil {
				return
			}
			util.Logger.Debug(fmt.Sprintf("%s exists only in permissions-v2, now deleted", permResouceId))
		}
	}
	return
}

func (r *MongoRepo) InsertFlow(flow lib.Flow) (err error) {
	flow.DateCreated = time.Now()
	flow.DateUpdated = time.Now()
	permissions := permV2Client.ResourcePermissions{
		GroupPermissions: map[string]permV2Client.PermissionsMap{},
		UserPermissions:  map[string]permV2Client.PermissionsMap{},
		RolePermissions:  map[string]permV2Model.PermissionsMap{},
	}
	SetDefaultPermissions(flow, permissions)
	result, err := Mongo().InsertOne(CTX, flow)
	id := result.InsertedID.(primitive.ObjectID).Hex()
	if err != nil {
		return err
	}
	_, err, _ = r.perm.SetPermission(permV2Client.InternalAdminToken, PermV2InstanceTopic, id, permissions)
	return
}

func (r *MongoRepo) UpdateFlow(id string, flow lib.Flow, _ string, auth string) (err error) {
	ok, err, _ := r.perm.CheckPermission(auth, PermV2InstanceTopic, id, permV2Client.Write)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New(MessageMissingRights)
	}

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}
	flow.DateUpdated = time.Now()
	_, err = Mongo().ReplaceOne(CTX, bson.M{"_id": objID}, flow)
	return
}

func (r *MongoRepo) DeleteFlow(id string, _ string, _ bool, auth string) (err error) {
	ok, err, _ := r.perm.CheckPermission(auth, PermV2InstanceTopic, id, permV2Client.Administrate)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New(MessageMissingRights)
	}

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}
	req := bson.M{"_id": objID}
	res := Mongo().FindOneAndDelete(CTX, req)
	if res.Err() != nil {
		return res.Err()
	}
	err, _ = r.perm.RemoveResource(permV2Client.InternalAdminToken, PermV2InstanceTopic, id)
	return
}

func (r *MongoRepo) All(userId string, admin bool, args map[string][]string, auth string) (response lib.FlowsResponse, err error) {
	opt := options.Find()
	for arg, value := range args {
		if len(value) == 0 {
			continue
		}

		switch arg {
		case "sort":
			sortFields := []string{"name", "dateCreated", "dateUpdated"}
			ord := strings.SplitN(value[0], ":", 2)
			if len(ord) == 2 {
				field, dir := ord[0], ord[1]
				if slices.Contains(sortFields, field) {
					order := int64(1)
					if dir == "desc" {
						order = -1
					}
					opt.SetSort(bson.M{field: order})
				}
			}
		case "limit":
			var limit int64
			limit, err = strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				return
			}
			if limit > 0 {
				opt.SetLimit(limit)
			}
		case "offset":
			var skip int64
			skip, err = strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				return
			}
			if skip > 0 {
				opt.SetSkip(skip)
			}
		}
	}

	andFilters := bson.A{}

	ids := []primitive.ObjectID{}
	var stringIds []string
	if !admin {
		stringIds, err, _ = r.perm.ListAccessibleResourceIds(auth, PermV2InstanceTopic, permV2Client.ListOptions{}, permV2Client.Read)
		if err != nil {
			return
		}
		for _, id := range stringIds {
			objID, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				return lib.FlowsResponse{}, err
			}
			ids = append(ids, objID)
		}

		andFilters = append(andFilters, bson.M{
			"$or": bson.A{
				bson.M{"_id": bson.M{"$in": ids}},
				bson.M{"userid": userId},
			},
		})
	}
	if val, ok := args["search"]; ok && len(val) > 0 {
		pattern := regexp.QuoteMeta(val[0])
		andFilters = append(andFilters, bson.M{
			"name": bson.M{
				"$regex":   pattern,
				"$options": "i",
			},
		})
	}

	if vals, ok := args["filter"]; ok {
		for _, raw := range vals {
			for _, f := range strings.Split(raw, "|") {

				parts := strings.SplitN(f, ":", 2)
				if len(parts) != 2 {
					continue
				}

				key := parts[0]
				values := strings.Split(parts[1], ",")
				if len(values) == 0 {
					continue
				}

				switch key {

				case "operator":
					andFilters = append(andFilters, bson.M{
						"model.cells": bson.M{
							"$elemMatch": bson.M{
								"type":       "senergy.NodeElement",
								"operatorid": bson.M{"$in": values},
							},
						},
					})

				default:
					fieldMap := map[string]string{}
					field, exists := fieldMap[key]
					if !exists {
						continue
					}
					andFilters = append(andFilters, bson.M{
						field: bson.M{"$in": values},
					})
				}
			}
		}
	}

	req := bson.M{}
	if len(andFilters) > 0 {
		req["$and"] = andFilters
	}

	var cur *mongo.Cursor

	cur, err = Mongo().Find(CTX, req, opt)
	if err != nil {
		return
	}

	response.Total, err = Mongo().CountDocuments(CTX, req)
	if err != nil {
		return
	}
	response.Flows = make([]lib.Flow, 0)

	err = cur.All(CTX, &response.Flows)
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			return
		}
	}(cur, CTX)

	if err != nil {
		return lib.FlowsResponse{}, err
	}
	return
}

func (r *MongoRepo) FindFlow(id, _, auth string) (flow lib.Flow, err error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return
	}

	ok, err, _ := r.perm.CheckPermission(auth, PermV2InstanceTopic, id, permV2Client.Read)
	if err != nil {
		return flow, err
	}
	if !ok {
		return flow, errors.New(MessageMissingRights)
	}

	err = Mongo().FindOne(CTX, bson.M{"_id": objID}).Decode(&flow)
	if err != nil {
		return
	}
	return
}

func (r *MongoRepo) GetOperatorFlowMapping() ([]lib.OperatorFlowCount, error) {
	pipeline := mongo.Pipeline{
		{{"$unwind", "$model.cells"}},
		{{"$match", bson.D{{"model.cells.type", "senergy.NodeElement"}}}},
		{{"$group", bson.D{
			{"_id", bson.D{
				{"flowId", "$_id"},
				{"operatorId", "$model.cells.operatorid"},
			}},
			{"count", bson.D{{"$sum", 1}}},
		}}},
		{{"$group", bson.D{
			{"_id", "$_id.operatorId"},
			{"flows", bson.D{{"$push", bson.D{
				{"flowId", "$_id.flowId"},
				{"count", "$count"},
			}}}},
		}}},
	}

	cursor, err := Mongo().Aggregate(CTX, pipeline)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err = cursor.Close(ctx)
		if err != nil {
			return
		}
	}(cursor, CTX)

	var results []lib.OperatorFlowCount
	if err = cursor.All(CTX, &results); err != nil {
		return nil, err
	}

	return results, nil
}
