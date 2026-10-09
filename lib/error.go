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

package lib

import "github.com/SENERGY-Platform/analytics-pipeline/lib"

type cError struct {
	err error
}

type InternalError struct {
	cError
}

type InputError struct {
	cError
}

type NotFoundError struct {
	cError
}

// StillInUseError refuses a deletion. FlowUsage is the pipelines that use the flow, Smart the
// smart service releases; each is nil when it does not use the flow.
type StillInUseError struct {
	*lib.FlowUsage
	Smart *SmartServiceUsage
	cError
}

// UsageUnavailableError means the smart-service-repository could not tell whether smart services
// use the flow. Its text reaches the client, so build it only from static strings.
type UsageUnavailableError struct {
	cError
}

type ExternalResourceError struct {
	cError
}

type ForbiddenError struct {
	cError
}

func (e *cError) Error() string {
	return e.err.Error()
}

func (e *cError) Unwrap() error {
	return e.err
}

func NewInternalError(err error) error {
	return &InternalError{cError{err: err}}
}

func NewInputError(err error) error {
	return &InputError{cError{err: err}}
}

func NewNotFoundError(err error) error {
	return &NotFoundError{cError{err: err}}
}

func NewStillInUseError(usage *lib.FlowUsage, smart *SmartServiceUsage, err error) error {
	return &StillInUseError{usage, smart, cError{err: err}}
}

// PipelineCount is the number of pipelines in the registry's answer, at least 1 for a non-nil
// answer: the registry only answers 200 for a flow that pipelines use.
func PipelineCount(usage *lib.FlowUsage) int {
	if usage == nil {
		return 0
	}
	return max(1, int(usage.Count), len(usage.PipelineIds))
}

func NewUsageUnavailableError(err error) error {
	return &UsageUnavailableError{cError{err: err}}
}

func NewExternalResourceError(err error) error {
	return &ExternalResourceError{cError{err: err}}
}

func NewForbiddenError(err error) error {
	return &ForbiddenError{cError{err: err}}
}
