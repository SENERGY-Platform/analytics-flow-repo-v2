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

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/pkg/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// legacyOwnerFilter leaves a userid of any other type alone, so it is neither lost nor copied into
// userId, where it would break decoding every flow listing.
var legacyOwnerFilter = bson.M{"userid": bson.M{"$exists": true, "$type": bson.A{"string", "null"}}}

// legacyOwnerUpdate moves the owner from userid to userId. A non-empty string userid fills a
// missing, null or empty userId; any other userId is kept, and a missing one stays missing.
var legacyOwnerUpdate = mongo.Pipeline{
	{{Key: "$set", Value: bson.D{{Key: "userId", Value: bson.D{{Key: "$cond", Value: bson.A{
		bson.D{{Key: "$and", Value: bson.A{
			bson.D{{Key: "$eq", Value: bson.A{bson.D{{Key: "$ifNull", Value: bson.A{"$userId", ""}}}, ""}}},
			bson.D{{Key: "$eq", Value: bson.A{bson.D{{Key: "$type", Value: "$userid"}}, "string"}}},
			bson.D{{Key: "$ne", Value: bson.A{"$userid", ""}}},
		}}},
		"$userid",
		bson.D{{Key: "$cond", Value: bson.A{
			bson.D{{Key: "$eq", Value: bson.A{bson.D{{Key: "$type", Value: "$userId"}}, "missing"}}},
			"$$REMOVE",
			"$userId",
		}}},
	}}}}}}},
	{{Key: "$unset", Value: "userid"}},
}

// migrateLegacyOwners moves the owner of flows stored under the legacy key userid to userId and
// removes userid. It touches no other field, dateUpdated included, and a repeat matches nothing.
func migrateLegacyOwners(ctx context.Context, coll *mongo.Collection) (matched, modified int64, err error) {
	if err = warnConflictingOwners(ctx, coll); err != nil {
		return
	}
	res, err := coll.UpdateMany(ctx, legacyOwnerFilter, legacyOwnerUpdate)
	if err != nil {
		return
	}
	matched, modified = res.MatchedCount, res.ModifiedCount
	if matched == 0 {
		util.Logger.Debug("no flow stores its owner under the legacy key userid")
	} else {
		util.Logger.Info("moved the flow owner from the legacy key userid to userId", "matched", matched, "modified", modified)
	}
	left, err := flowIds(ctx, coll, bson.M{"userid": bson.M{"$exists": true}})
	if err != nil {
		return
	}
	if len(left) > 0 {
		util.Logger.Warn("flows keep the legacy key userid with a value that is neither a string nor null", "count", len(left), "flow_ids", left)
	}
	return
}

// warnConflictingOwners logs flows that store two different owners; the migration keeps userId,
// which is the owner the service has read so far, and drops userid.
func warnConflictingOwners(ctx context.Context, coll *mongo.Collection) error {
	cur, err := coll.Find(ctx, bson.M{
		"userid": bson.M{"$type": "string", "$ne": ""},
		"userId": bson.M{"$type": "string", "$ne": ""},
		"$expr":  bson.M{"$ne": bson.A{"$userId", "$userid"}},
	}, options.Find().SetProjection(bson.M{"_id": 1, "userId": 1, "userid": 1}))
	if err != nil {
		return err
	}
	var conflicts []struct {
		Id     primitive.ObjectID `bson:"_id"`
		Kept   string             `bson:"userId"`
		Legacy string             `bson:"userid"`
	}
	if err = cur.All(ctx, &conflicts); err != nil {
		return err
	}
	for _, c := range conflicts {
		util.Logger.Warn("flow stores two different owners, keeping userId and dropping the legacy userid",
			"flow_id", c.Id.Hex(), "user_id", c.Kept, "dropped_user_id", c.Legacy)
	}
	return nil
}

func flowIds(ctx context.Context, coll *mongo.Collection, filter bson.M) ([]string, error) {
	cur, err := coll.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Id primitive.ObjectID `bson:"_id"`
	}
	if err = cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.Id.Hex())
	}
	return ids, nil
}
