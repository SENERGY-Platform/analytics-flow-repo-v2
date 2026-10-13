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
	"errors"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/analytics-flow-repo-v2/lib"
	operator_repo "github.com/SENERGY-Platform/analytics-operator-repo-v2/lib"
)

type fakeOperators struct {
	operators map[string]operator_repo.Operator
	err       error
	asked     []string
}

func (f *fakeOperators) GetOperator(id, _, _ string) (operator_repo.Operator, error) {
	f.asked = append(f.asked, id)
	if f.err != nil {
		return operator_repo.Operator{}, f.err
	}
	return f.operators[id], nil
}

func versionedOperator() operator_repo.Operator {
	cost := int64(7)
	return operator_repo.Operator{
		Name:           "filter",
		Image:          "registry/filter:newest",
		DeploymentType: "local",
		Cost:           &cost,
		Versions: []operator_repo.OperatorVersion{
			{Version: "v1.0.0", Image: "registry/filter:1.0.0", Deprecated: true},
			{Version: "v2.0.0", Image: "registry/filter:2.0.0"},
		},
	}
}

func nodeFlow(id string, operatorId *string, pin *string) *lib.Flow {
	return &lib.Flow{Model: lib.Model{Cells: []lib.Cell{
		{Type: "link", Id: "link-1"},
		{Type: "senergy.NodeElement", Id: id, OperatorId: operatorId, OperatorVersion: pin, InPorts: []string{"in"}},
	}}}
}

func str(s string) *string { return &s }

func TestValidateOperatorsWithoutPinUsesTheOperatorImage(t *testing.T) {
	for name, pin := range map[string]*string{"nil": nil, "empty": str("")} {
		t.Run(name, func(t *testing.T) {
			r := &Repo{operatorRepo: &fakeOperators{operators: map[string]operator_repo.Operator{"op-1": versionedOperator()}}}
			flow := nodeFlow("n1", str("op-1"), pin)

			if err := r.validateOperators(flow, "user-a", "Bearer a"); err != nil {
				t.Fatal(err)
			}
			cell := flow.Model.Cells[1]
			if *cell.Image != "registry/filter:newest" || *cell.Name != "filter" || *cell.DeploymentType != "local" || *cell.Cost != 7 {
				t.Errorf("cell = image %q, name %q, deployment %q, cost %d", *cell.Image, *cell.Name, *cell.DeploymentType, *cell.Cost)
			}
		})
	}
}

func TestValidateOperatorsWithPinUsesTheVersionImage(t *testing.T) {
	r := &Repo{operatorRepo: &fakeOperators{operators: map[string]operator_repo.Operator{"op-1": versionedOperator()}}}
	flow := nodeFlow("n1", str("op-1"), str("v2.0.0"))

	if err := r.validateOperators(flow, "user-a", ""); err != nil {
		t.Fatal(err)
	}
	cell := flow.Model.Cells[1]
	if *cell.Image != "registry/filter:2.0.0" {
		t.Errorf("image = %q, want the image of v2.0.0", *cell.Image)
	}
	if *cell.Name != "filter" || *cell.DeploymentType != "local" || *cell.Cost != 7 {
		t.Errorf("name, deployment type and cost must come from the operator: %q, %q, %d", *cell.Name, *cell.DeploymentType, *cell.Cost)
	}
	if *cell.OperatorVersion != "v2.0.0" || len(cell.InPorts) != 1 {
		t.Errorf("the pin and the ports must stay as sent: %q, %v", *cell.OperatorVersion, cell.InPorts)
	}
}

func TestValidateOperatorsAcceptsADeprecatedPin(t *testing.T) {
	r := &Repo{operatorRepo: &fakeOperators{operators: map[string]operator_repo.Operator{"op-1": versionedOperator()}}}
	flow := nodeFlow("n1", str("op-1"), str("v1.0.0"))

	if err := r.validateOperators(flow, "user-a", ""); err != nil {
		t.Fatal(err)
	}
	if got := *flow.Model.Cells[1].Image; got != "registry/filter:1.0.0" {
		t.Errorf("image = %q, want the image of v1.0.0", got)
	}
}

func TestValidateOperatorsRefusesAnUnknownLabel(t *testing.T) {
	r := &Repo{operatorRepo: &fakeOperators{operators: map[string]operator_repo.Operator{"op-1": versionedOperator()}}}

	err := r.validateOperators(nodeFlow("n1", str("op-1"), str("v9.9.9")), "user-a", "")

	var input *lib.InputError
	if !errors.As(err, &input) {
		t.Fatalf("err = %v, want an input error", err)
	}
	for _, want := range []string{"n1", "v9.9.9"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

func TestValidateOperatorsRefusesAPinOnAnOperatorWithoutVersions(t *testing.T) {
	op := versionedOperator()
	op.Versions = nil
	r := &Repo{operatorRepo: &fakeOperators{operators: map[string]operator_repo.Operator{"op-1": op}}}

	err := r.validateOperators(nodeFlow("n1", str("op-1"), str("v1.0.0")), "user-a", "")

	if !errors.As(err, new(*lib.InputError)) {
		t.Fatalf("err = %v, want an input error", err)
	}
}

func TestValidateOperatorsRefusesANodeWithoutOperatorId(t *testing.T) {
	ops := &fakeOperators{}
	r := &Repo{operatorRepo: ops}

	err := r.validateOperators(nodeFlow("n1", nil, nil), "user-a", "")

	if !errors.As(err, new(*lib.InputError)) {
		t.Fatalf("err = %v, want an input error", err)
	}
	if !strings.Contains(err.Error(), "n1") {
		t.Errorf("error %q does not name the node", err)
	}
	if len(ops.asked) != 0 {
		t.Error("the operator service was asked without an operator id")
	}
}

func TestValidateOperatorsReportsAnUnreachableOperatorService(t *testing.T) {
	r := &Repo{operatorRepo: &fakeOperators{err: errors.New("down")}}

	err := r.validateOperators(nodeFlow("n1", str("op-1"), nil), "user-a", "")

	if !errors.As(err, new(*lib.ExternalResourceError)) {
		t.Fatalf("err = %v, want an external resource error", err)
	}
}

func TestValidateOperatorsRefusesAnEmptyOperatorId(t *testing.T) {
	ops := &fakeOperators{}
	r := &Repo{operatorRepo: ops}

	err := r.validateOperators(nodeFlow("n1", str(""), nil), "user-a", "")

	if !errors.As(err, new(*lib.InputError)) {
		t.Fatalf("err = %v, want an input error", err)
	}
	if len(ops.asked) != 0 {
		t.Error("the operator service was asked with an empty operator id")
	}
}

func TestValidateOperatorsDropsAnEmptyPin(t *testing.T) {
	ops := &fakeOperators{operators: map[string]operator_repo.Operator{"op-1": versionedOperator()}}
	r := &Repo{operatorRepo: ops}
	flow := nodeFlow("n1", str("op-1"), str(""))

	if err := r.validateOperators(flow, "user-a", ""); err != nil {
		t.Fatal(err)
	}

	node := flow.Model.Cells[1]
	if node.OperatorVersion != nil {
		t.Errorf("operatorVersion = %q, want it dropped", *node.OperatorVersion)
	}
	if *node.Image != "registry/filter:newest" {
		t.Errorf("image = %q, want the operator image", *node.Image)
	}
}
