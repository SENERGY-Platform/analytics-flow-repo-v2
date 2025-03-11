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

package models

import (
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type FlowsResponse struct {
	Flows []Flow `json:"flows"`
	Total int64  `json:"total"`
}
type Flow struct {
	Id          *primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name        string              `json:"name,omitempty"`
	Description *string             `json:"description,omitempty"`
	Model       Model               `json:"model,omitempty"`
	Image       *string             `json:"image,omitempty"`
	Share       *Share              `json:"share,omitempty"`
	UserId      string              `bson:"userId,omitempty" json:"userId,omitempty"`
	DateCreated time.Time           `bson:"dateCreated,omitempty" json:"dateCreated,omitempty"`
	DateUpdated time.Time           `bson:"dateUpdated,omitempty" json:"dateUpdated,omitempty"`
}

type Share struct {
	List  *bool `json:"list,omitempty"`
	Read  *bool `json:"read,omitempty"`
	Write *bool `json:"write,omitempty"`
}

type Model struct {
	Cells []Cell `json:"cells,omitempty"`
}

type Cell struct {
	Type           string         `json:"type,omitempty"`
	InPorts        []string       `json:"inPorts,omitempty"`
	OutPorts       []string       `json:"outPorts,omitempty"`
	Name           *string        `json:"name,omitempty"`
	Image          *string        `json:"image,omitempty"`
	OperatorId     *string        `json:"operatorId,omitempty"`
	Position       *CellPosition  `json:"position,omitempty"`
	Source         *CellLink      `json:"source,omitempty"`
	Target         *CellLink      `json:"target,omitempty"`
	Id             string         `json:"id,omitempty"`
	Config         *[]ConfigValue `json:"config,omitempty"`
	Cost           *int64         `json:"cost,omitempty"`
	DeploymentType *string        `json:"deploymentType,omitempty"`
}

type CellPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CellLink struct {
	Id     string `json:"id"`
	Magnet string `json:"magnet"`
	Port   string `json:"port"`
}

type ConfigValue struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

func SetDefaultPermissions(instance Flow, permissions permV2Client.ResourcePermissions) {
	permissions.UserPermissions[instance.UserId] = permV2Client.PermissionsMap{
		Read:         true,
		Write:        true,
		Execute:      true,
		Administrate: true,
	}
}
