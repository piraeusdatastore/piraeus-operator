package linstorsatelliteset

import (
	"reflect"
	"testing"

	lapi "github.com/LINBIT/golinstor/client"
)

func TestFilterNode(t *testing.T) {
	var tableTest = []struct {
		raw        []lapi.Resource
		filterNode string
		filtered   []lapi.Resource
	}{
		{
			[]lapi.Resource{
				{Name: "test4", NodeName: "node4"},
				{Name: "test3", NodeName: "node3"},
				{Name: "test2", NodeName: "node2"},
				{Name: "test1", NodeName: "node1"},
			},
			"node4",
			[]lapi.Resource{
				{Name: "test4", NodeName: "node4"},
			},
		},
		{
			[]lapi.Resource{
				{Name: "test4", NodeName: "node4"},
				{Name: "test3", NodeName: "node3"},
				{Name: "test2", NodeName: "node2"},
				{Name: "test1", NodeName: "node1"},
			},
			"node1",
			[]lapi.Resource{
				{Name: "test1", NodeName: "node1"},
			},
		},
		{
			[]lapi.Resource{
				{Name: "test4", NodeName: "node2"},
				{Name: "test3", NodeName: "node2"},
				{Name: "test2", NodeName: "node2"},
				{Name: "test1", NodeName: "node2"},
			},
			"node2",
			[]lapi.Resource{
				{Name: "test4", NodeName: "node2"},
				{Name: "test3", NodeName: "node2"},
				{Name: "test2", NodeName: "node2"},
				{Name: "test1", NodeName: "node2"},
			},
		},
		{
			[]lapi.Resource{
				{Name: "test0", NodeName: "node0"},
				{Name: "test0", NodeName: "node1"},
				{Name: "test1", NodeName: "node1"},
				{Name: "test0", NodeName: "node2"},
			},
			"node1",
			[]lapi.Resource{
				{Name: "test0", NodeName: "node1"},
				{Name: "test1", NodeName: "node1"},
			},
		},
		{
			[]lapi.Resource{},
			"node4",
			[]lapi.Resource{},
		},
		{
			[]lapi.Resource{
				{Name: "test0", NodeName: "node0"},
				{Name: "test0", NodeName: "node1"},
				{Name: "test1", NodeName: "node1"},
				{Name: "test0", NodeName: "node2"},
			},
			"node0",
			[]lapi.Resource{
				{Name: "test0", NodeName: "node0"},
			},
		},
		{
			[]lapi.Resource{
				{Name: "test0", NodeName: "node0"},
				{Name: "test0", NodeName: "node1"},
				{Name: "test1", NodeName: "node1"},
				{Name: "test0", NodeName: "node2"},
			},
			"fake-node",
			[]lapi.Resource{},
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

func TestContains(t *testing.T) {
	var tableTest = []struct {
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
		actual := contains(tt.in, tt.has)

		if tt.expected != actual {
			t.Errorf("\nexpected contains(%+v, %s)\nto be\n\t%t\ngot\n\t%t",
				tt.in, tt.has, tt.expected, actual)
		}
	}
}

func TestRemove(t *testing.T) {
	var tableTest = []struct {
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
