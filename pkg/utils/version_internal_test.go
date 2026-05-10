package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
)

func TestParseKubernetesVersionPart(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name     string
		version  string
		expected int
		wantErr  bool
	}{
		{
			name:     "plain-number",
			version:  "34",
			expected: 34,
		},
		{
			name:     "dirty-git-version",
			version:  "34+",
			expected: 34,
		},
		{
			name:    "empty",
			version: "",
			wantErr: true,
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual, err := parseKubernetesVersionPart(tcase.version)
			if tcase.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAPIDiscoveryClientServerVersionHandlesDirtyMinorVersion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/version", r.URL.Path)
		err := json.NewEncoder(w).Encode(apimachineryversion.Info{
			Major: "1",
			Minor: "34+",
		})
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	fallback := &APIVersion{Major: 1, Minor: 20}
	client := NewAPIDiscoveryClient(&rest.Config{Host: srv.URL}, fallback)

	assert.Equal(t, &APIVersion{Major: 1, Minor: 34}, client.ServerVersion())
}
