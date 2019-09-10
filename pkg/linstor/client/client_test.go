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

package client

import (
	"reflect"
	"testing"

	lapi "github.com/LINBIT/golinstor/client"
)

func TestFilterNode(t *testing.T) {
	var tableTest = []struct {
		raw        []lapi.ResourceWithVolumes
		filterNode string
		filtered   []lapi.ResourceWithVolumes
	}{
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node4"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node3"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
			"node4",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node4"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node4"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node3"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
			"node1",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node2"}},
			},
			"node2",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node2"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node2"}},
			},
			"node1",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{},
			"node4",
			[]lapi.ResourceWithVolumes{},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node2"}},
			},
			"node0",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node2"}},
			},
			"fake-node",
			[]lapi.ResourceWithVolumes{},
		},
	}

	for _, tt := range tableTest {
		actual := filterNodes(tt.raw, tt.filterNode)

		if !reflect.DeepEqual(tt.filtered, actual) {
			// Structs are printed without field names for a more compact comparison.
			t.Errorf("\nexpected\n\t%v\nto filter into\n\t%v\ngot\n\t%v",
				tt.raw, tt.filtered, actual)
		}
	}
}
