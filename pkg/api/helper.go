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

package api

import (
	"errors"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	"go.mongodb.org/mongo-driver/mongo"
)

func handleError(err error) error {
	var ie *lib.ForbiddenError
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = lib.NewNotFoundError(errors.New(MessageNotFound))
	} else if errors.As(err, &ie) {
		err = lib.NewForbiddenError(errors.New(MessageForbidden))
	} else {
		err = lib.NewInternalError(errors.New(MessageSomethingWrong))
	}
	return err
}
