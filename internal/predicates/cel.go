/*
Copyright 2026 The provider-anthropic Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package predicates provides client-side predicate evaluation for
// Observed<Resource>Collection filters.
package predicates

import (
	celgo "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/pkg/errors"
)

const (
	errCelQueryFailedToCompile           = "failed to compile query"
	errCelQueryReturnTypeNotBool         = "celQuery does not return a bool type"
	errCelQueryFailedToCreateProgram     = "failed to create program from the cel query"
	errCelQueryFailedToEvalProgram       = "failed to eval the program"
	errCelQueryFailedToCreateEnvironment = "cel query failed to create environment"
)

// PredicateDeriveFromCelQuery evaluates celQuery against obj, returning true
// if obj should be included. The variable name in CEL expressions is "atProvider".
// obj should be the JSON-decoded API response item (map[string]any). Use
// dot notation for field access: atProvider.name == "foo".
func PredicateDeriveFromCelQuery(celQuery string, obj map[string]any) (bool, error) {
	env, err := celgo.NewEnv(
		celgo.Variable("atProvider", celgo.AnyType),
	)
	if err != nil {
		return false, errors.Wrap(err, errCelQueryFailedToCreateEnvironment)
	}

	ast, iss := env.Compile(celQuery)
	if iss.Err() != nil {
		return false, errors.Wrap(iss.Err(), errCelQueryFailedToCompile)
	}

	if !ast.OutputType().IsExactType(celgo.BoolType) {
		return false, errors.New(errCelQueryReturnTypeNotBool)
	}

	program, err := env.Program(ast)
	if err != nil {
		return false, errors.Wrap(err, errCelQueryFailedToCreateProgram)
	}

	val, _, err := program.Eval(map[string]any{
		"atProvider": obj,
	})
	if err != nil {
		return false, errors.Wrap(err, errCelQueryFailedToEvalProgram)
	}

	return val == celtypes.True, nil
}
