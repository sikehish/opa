// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"github.com/open-policy-agent/opa/ast"
)

var (
	emptyArr = ast.ArrayTerm()
)

func evalWalk(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	input := operands[0]

	if pathIsWildcard(operands) {
		// When the path assignment is a wildcard: walk(input, [_, value])
		// we may skip the path construction entirely, and simply return
		// same pointer in each iteration. This is a much more efficient
		// path when only the values are needed.
		return walkNoPath(input, iter)
	}

	filter := getOutputPath(operands)
	return walk(filter, nil, input, iter)
}

func walk(filter, path *ast.Array, input *ast.Term, iter func(*ast.Term) error) error {
	if filter == nil || filter.Len() == 0 {
		if path == nil {
			path = ast.NewArray()
		}

		if err := iter(ast.ArrayTerm(ast.NewTerm(path.Copy()), input)); err != nil {
			return err
		}
	}

	if filter != nil && filter.Len() > 0 {
		key := filter.Elem(0)
		filter = filter.Slice(1, -1)
		if key.IsGround() {
			if term := input.Get(key); term != nil {
				path = pathAppend(path, key)
				return walk(filter, path, term, iter)
			}
			return nil
		}
	}

	switch v := input.Value.(type) {
	case *ast.Array:
		for i := 0; i < v.Len(); i++ {
			if err := walk(filter, pathAppend(path, ast.InternedIntNumberTerm(i)), v.Elem(i), iter); err != nil {
				return err
			}
		}
	case ast.Object:
		return v.Iter(func(k, v *ast.Term) error {
			if err := walk(filter, pathAppend(path, k), v, iter); err != nil {
				return err
			}
			return nil
		})
	case ast.Set:
		for _, elem := range v.Slice() {
			if err := walk(filter, pathAppend(path, elem), elem, iter); err != nil {
				return err
			}
		}
	}

	return nil
}

func walkNoPath(input *ast.Term, iter func(*ast.Term) error) error {
	if err := iter(ast.ArrayTerm(emptyArr, input)); err != nil {
		return err
	}

	switch v := input.Value.(type) {
	case ast.Object:
		return v.Iter(func(_, v *ast.Term) error {
			return walkNoPath(v, iter)
		})
	case *ast.Array:
		for i := 0; i < v.Len(); i++ {
			if err := walkNoPath(v.Elem(i), iter); err != nil {
				return err
			}
		}
	case ast.Set:
		for _, elem := range v.Slice() {
			if err := walkNoPath(elem, iter); err != nil {
				return err
			}
		}
	}

	return nil
}

func pathAppend(path *ast.Array, key *ast.Term) *ast.Array {
	if path == nil {
		return ast.NewArray(key)
	}

	return path.Append(key)
}

func getOutputPath(operands []*ast.Term) *ast.Array {
	if len(operands) == 2 {
		if arr, ok := operands[1].Value.(*ast.Array); ok && arr.Len() == 2 {
			if path, ok := arr.Elem(0).Value.(*ast.Array); ok {
				return path
			}
		}
	}
	return nil
}

func pathIsWildcard(operands []*ast.Term) bool {
	if len(operands) == 2 {
		if arr, ok := operands[1].Value.(*ast.Array); ok && arr.Len() == 2 {
			if v, ok := arr.Elem(0).Value.(ast.Var); ok {
				return v.IsWildcard()
			}
		}
	}
	return false
}

func init() {
	RegisterBuiltinFunc(ast.WalkBuiltin.Name, evalWalk)
}
