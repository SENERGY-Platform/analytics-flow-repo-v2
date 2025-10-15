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
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/models"
	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	permV2Model "github.com/SENERGY-Platform/permissions-v2/pkg/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FlowRepository interface {
	InsertFlow(flow models.Flow) (err error)
	UpdateFlow(id string, flow models.Flow, userId string, auth string) (err error)
	DeleteFlow(id string, userId string, admin bool, auth string) (err error)
	All(userId string, admin bool, args map[string][]string, auth string) (response models.FlowsResponse, err error)
	FindFlow(id string, userId string, auth string) (flow models.Flow, err error)
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
		models.SetDefaultPermissions(flow, permissions)

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

func (r *MongoRepo) InsertFlow(flow models.Flow) (err error) {
	flow.DateCreated = time.Now()
	flow.DateUpdated = time.Now()
	permissions := permV2Client.ResourcePermissions{
		GroupPermissions: map[string]permV2Client.PermissionsMap{},
		UserPermissions:  map[string]permV2Client.PermissionsMap{},
		RolePermissions:  map[string]permV2Model.PermissionsMap{},
	}
	models.SetDefaultPermissions(flow, permissions)
	result, err := Mongo().InsertOne(CTX, flow)
	id := result.InsertedID.(primitive.ObjectID).Hex()
	if err != nil {
		return err
	}
	_, err, _ = r.perm.SetPermission(permV2Client.InternalAdminToken, PermV2InstanceTopic, id, permissions)
	return
}

func (r *MongoRepo) UpdateFlow(id string, flow models.Flow, userId string, auth string) (err error) {
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

func (r *MongoRepo) DeleteFlow(id string, userId string, admin bool, auth string) (err error) {
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
	err, _ = r.perm.RemoveResource(auth, PermV2InstanceTopic, id)
	return
}

func (r *MongoRepo) All(userId string, admin bool, args map[string][]string, auth string) (response models.FlowsResponse, err error) {
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
	var req = bson.M{}
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
				return models.FlowsResponse{}, err
			}
			ids = append(ids, objID)
		}
		req = bson.M{
			"$or": []interface{}{
				bson.M{"_id": bson.M{"$in": ids}},
				bson.M{"userId": userId},
			}}
		if val, ok := args["search"]; ok {
			req = bson.M{
				"name": bson.M{"$regex": val[0]},
				"$or": []interface{}{
					bson.M{"_id": bson.M{"$in": ids}},
					bson.M{"userId": userId},
				}}
		}
	}
	cur, err = Mongo().Find(CTX, req, opt)
	if err != nil {
		return
	}

	req = bson.M{}
	if !admin {
		req = bson.M{
			"$or": []interface{}{
				bson.M{"_id": bson.M{"$in": ids}},
				bson.M{"userId": userId},
			}}
		if val, ok := args["search"]; ok {
			req = bson.M{
				"name": bson.M{"$regex": val[0]},
				"$or": []interface{}{
					bson.M{"_id": bson.M{"$in": ids}},
					bson.M{"userId": userId},
				}}
		}
	}

	response.Total, err = Mongo().CountDocuments(CTX, req)
	if err != nil {
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

func (r *MongoRepo) FindFlow(id string, userId string, auth string) (flow models.Flow, err error) {
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
