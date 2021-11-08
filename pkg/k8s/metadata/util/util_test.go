/*
Piraeus Operator
Copyright 2019 LINBIT USA, LLC.

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

package util

import (
	"reflect"
	"testing"
)

func TestContains(t *testing.T) {
	tableTest := []struct {
		in       []string
		has      string
		expected bool
	}{
		{[]string{"foo", "bar", "baz"}, "bar", true},
		{[]string{"foo", "bar", "baz"}, "foo", true},
		{[]string{"foo", "bar", "baz"}, "baz", true},
		{[]string{"foo", "bar", "baz"}, "banana", false},
		{[]string{"foo", "bar", "baz"}, "", false},
		{[]string{"foo", "bar", ""}, "", true},
		{[]string{}, "banana", false},
	}

	for _, tt := range tableTest {
		actual := SliceContains(tt.in, tt.has)

		if tt.expected != actual {
			t.Errorf("\nexpected contains(%+v, %s)\nto be\n\t%t\ngot\n\t%t",
				tt.in, tt.has, tt.expected, actual)
		}
	}
}

func TestRemove(t *testing.T) {
	tableTest := []struct {
		in       []string
		remove   string
		expected []string
	}{
		{[]string{"foo", "bar", "baz"}, "bar", []string{"foo", "baz"}},
		{[]string{"foo", "bar", "baz"}, "baz", []string{"foo", "bar"}},
		{[]string{"foo", "bar", "baz"}, "foo", []string{"bar", "baz"}},
		{[]string{"foo", "bar", "baz"}, "potato", []string{"foo", "bar", "baz"}},
		{[]string{"foo", "bar", "baz"}, "", []string{"foo", "bar", "baz"}},
		{[]string{}, "bar", []string{}},
	}

	for _, tt := range tableTest {
		actual := remove(tt.in, tt.remove)

		if !reflect.DeepEqual(tt.expected, actual) {
			t.Errorf("\nexpected contains(%+v, %s)\nto be\n\t%+v\ngot\n\t%+v",
				tt.in, tt.remove, tt.expected, actual)
		}
	}
}
