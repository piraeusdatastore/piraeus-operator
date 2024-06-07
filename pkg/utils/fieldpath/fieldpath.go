/*
Copyright 2015 The Kubernetes Authors.

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

package fieldpath

import (
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ExtractFieldPath extracts the field(s) from the given object
// and returns the value as string, along with the expanded value of any
// wildcard values.
//
// If a wildcard path was given, keys is not nil.
func ExtractFieldPath(obj interface{}, fieldPath string) ([]string, []string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, nil, err
	}

	if path, subscript, ok := SplitMaybeSubscriptedPath(fieldPath); ok {
		switch path {
		case "metadata.annotations":
			if strings.HasSuffix(subscript, "*") {
				keys, vals := mapToSlices(filterPrefix(accessor.GetAnnotations(), subscript[:len(subscript)-1]))
				return vals, keys, nil
			}

			if errs := validation.IsQualifiedName(strings.ToLower(subscript)); len(errs) != 0 {
				return nil, nil, fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}

			val, ok := accessor.GetAnnotations()[subscript]
			if ok {
				return []string{val}, nil, nil
			}

			return nil, nil, nil
		case "metadata.labels":
			if strings.HasSuffix(subscript, "*") {
				keys, vals := mapToSlices(filterPrefix(accessor.GetLabels(), subscript[:len(subscript)-1]))
				return vals, keys, nil
			}

			if errs := validation.IsQualifiedName(subscript); len(errs) != 0 {
				return nil, nil, fmt.Errorf("invalid key subscript in %s: %s", fieldPath, strings.Join(errs, ";"))
			}

			val, ok := accessor.GetLabels()[subscript]
			if ok {
				return []string{val}, nil, nil
			}

			return nil, nil, nil
		default:
			return nil, nil, fmt.Errorf("fieldPath %q does not support subscript", fieldPath)
		}
	}

	switch fieldPath {
	case "metadata.annotations":
		keys, values := mapToSlices(accessor.GetAnnotations())
		return values, keys, nil
	case "metadata.labels":
		keys, values := mapToSlices(accessor.GetLabels())
		return values, keys, nil
	case "metadata.name":
		return []string{accessor.GetName()}, nil, nil
	case "metadata.namespace":
		return []string{accessor.GetNamespace()}, nil, nil
	case "metadata.uid":
		return []string{string(accessor.GetUID())}, nil, nil
	}

	return nil, nil, fmt.Errorf("unsupported fieldPath: %v", fieldPath)
}

// filterPrefix returns all key-values where the key has the prefix.
// The new key will be the rest of the key.
func filterPrefix(m map[string]string, prefix string) map[string]string {
	result := map[string]string{}
	for k, v := range m {
		if strings.HasPrefix(k, prefix) {
			result[k[len(prefix):]] = v
		}
	}

	return result
}

func mapToSlices(m map[string]string) ([]string, []string) {
	keys := maps.Keys(m)
	slices.Sort(keys)

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, m[key])
	}

	return keys, values
}

// SplitMaybeSubscriptedPath checks whether the specified fieldPath is
// subscripted, and
//   - if yes, this function splits the fieldPath into path and subscript, and
//     returns (path, subscript, true).
//   - if no, this function returns (fieldPath, "", false).
//
// Example inputs and outputs:
//
//	"metadata.annotations['myKey']" --> ("metadata.annotations", "myKey", true)
//	"metadata.annotations['a[b]c']" --> ("metadata.annotations", "a[b]c", true)
//	"metadata.labels['']"           --> ("metadata.labels", "", true)
//	"metadata.labels"               --> ("metadata.labels", "", false)
func SplitMaybeSubscriptedPath(fieldPath string) (string, string, bool) {
	if !strings.HasSuffix(fieldPath, "']") {
		return fieldPath, "", false
	}
	s := strings.TrimSuffix(fieldPath, "']")
	parts := strings.SplitN(s, "['", 2)
	if len(parts) < 2 {
		return fieldPath, "", false
	}
	if len(parts[0]) == 0 {
		return fieldPath, "", false
	}
	return parts[0], parts[1], true
}
