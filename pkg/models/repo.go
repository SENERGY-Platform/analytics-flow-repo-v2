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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type FlowsResponse struct {
	Flows []Flow `json:"flows"`
	Total int64  `json:"total"`
}
type Flow struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Model       Model              `json:"model"`
	Image       string             `json:"image"`
	Share       Share              `json:"share"`
	UserId      string             `json:"userId"`
	DateCreated time.Time          `json:"dateCreated"`
	DateUpdated time.Time          `json:"dateUpdated"`
}

type Share struct {
	List  bool `json:"list"`
	Read  bool `json:"read"`
	Write bool `json:"write"`
}

type Model struct {
	Cells []Cell `json:"cells"`
}

type Cell struct {
	Type           string        `json:"type"`
	InPorts        []string      `json:"inPorts"`
	OutPorts       []string      `json:"outPorts"`
	Name           *string       `json:"name"`
	Image          *string       `json:"image"`
	OperatorId     *string       `json:"operatorId"`
	Position       *CellPosition `json:"position"`
	Source         *CellLink     `json:"source"`
	Target         *CellLink     `json:"target"`
	Id             string        `json:"id"`
	Cost           *int64        `json:"cost"`
	DeploymentType *string       `json:"deploymentType"`
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
