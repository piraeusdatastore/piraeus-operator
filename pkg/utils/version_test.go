package utils_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

func TestAPIVersion(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		a        utils.APIVersion
		b        utils.APIVersion
		expected int
	}{
		{
			a:        utils.APIVersion{Major: 1, Minor: 20},
			b:        utils.APIVersion{Major: 1, Minor: 20},
			expected: 0,
		},
		{
			a:        utils.APIVersion{Major: 1, Minor: 20},
			b:        utils.APIVersion{Major: 1, Minor: 19},
			expected: 1,
		},
		{
			a:        utils.APIVersion{Major: 1, Minor: 20},
			b:        utils.APIVersion{Major: 1, Minor: 21},
			expected: -1,
		},
		{
			a:        utils.APIVersion{Major: 1, Minor: 20},
			b:        utils.APIVersion{Major: 2, Minor: 19},
			expected: -1,
		},
		{
			a:        utils.APIVersion{Major: 1, Minor: 20},
			b:        utils.APIVersion{Major: 0, Minor: 21},
			expected: 1,
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(fmt.Sprintf("%s<=>%s", &tcase.a, &tcase.b), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tcase.expected, tcase.a.Compare(&tcase.b))
		})
	}
}
