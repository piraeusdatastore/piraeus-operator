package linstorhelper_test

import (
	"testing"

	lclient "github.com/LINBIT/golinstor/client"
	"github.com/stretchr/testify/assert"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
)

func TestMakePropertiesModification(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name     string
		current  map[string]string
		expected map[string]string
		result   *lclient.GenericPropsModify
	}{
		{
			name:   "empty",
			result: nil,
		},
		{
			name:    "keep-existing-if-empty",
			current: map[string]string{"old": "v0"},
			result:  nil,
		},
		{
			name:    "keep-existing-on-delete",
			current: map[string]string{"foo": "v1", "bar": "v2", linstorhelper.LastApplyProperty: "[\"foo\"]"},
			result: &lclient.GenericPropsModify{
				OverrideProps: map[string]string{},
				DeleteProps:   []string{"foo", linstorhelper.LastApplyProperty},
			},
		},
		{
			name:     "initial",
			expected: map[string]string{"foo": "v1", "bar": "v2"},
			result: &lclient.GenericPropsModify{
				OverrideProps: map[string]string{
					"foo":                           "v1",
					"bar":                           "v2",
					linstorhelper.LastApplyProperty: "[\"bar\",\"foo\"]",
				},
			},
		},
		{
			name:     "keep-existing",
			current:  map[string]string{"old": "v0"},
			expected: map[string]string{"foo": "v1", "bar": "v2"},
			result: &lclient.GenericPropsModify{
				OverrideProps: map[string]string{
					"foo":                           "v1",
					"bar":                           "v2",
					linstorhelper.LastApplyProperty: "[\"bar\",\"foo\"]",
				},
			},
		},
		{
			name:     "unchanged",
			current:  map[string]string{"foo": "v1", "bar": "v2", linstorhelper.LastApplyProperty: "[\"bar\",\"foo\"]"},
			expected: map[string]string{"foo": "v1", "bar": "v2"},
			result:   nil,
		},
		{
			name:     "delete-newly-not-set",
			current:  map[string]string{"foo": "v1", "bar": "v2", linstorhelper.LastApplyProperty: "[\"bar\",\"foo\"]"},
			expected: map[string]string{"foo": "v1"},
			result: &lclient.GenericPropsModify{
				OverrideProps: map[string]string{
					linstorhelper.LastApplyProperty: "[\"foo\"]",
				},
				DeleteProps: []string{"bar"},
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			diff := linstorhelper.MakePropertiesModification(tcase.current, tcase.expected)
			assert.Equal(t, tcase.result, diff)
		})
	}
}

func TestUpdateLastApplyProperty(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name   string
		props  map[string]string
		result string
	}{
		{
			name:   "empty",
			result: "",
		},
		{
			name: "some-vals",
			props: map[string]string{
				"foo": "val1",
				"bar": "val2",
			},
			result: "[\"bar\",\"foo\"]",
		},
		{
			name: "some-vals-with-preexisting",
			props: map[string]string{
				"foo1":                          "val1",
				"bar2":                          "val2",
				linstorhelper.LastApplyProperty: "[\"bar\",\"foo\"]",
			},
			result: "[\"bar2\",\"foo1\"]",
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual := linstorhelper.UpdateLastApplyProperty(tcase.props)
			assert.Equal(t, tcase.result, actual[linstorhelper.LastApplyProperty])
		})
	}
}
