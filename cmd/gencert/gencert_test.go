package main

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testNames = []string{
		"piraeus-operator-webhook-service.piraeus-datastore.svc",
		"piraeus-operator-webhook-service.piraeus-datastore.svc.cluster.local",
	}
	testCert = x509.Certificate{
		DNSNames:  testNames,
		NotAfter:  time.Date(2023, 6, 21, 13, 14, 15, 0, time.UTC),
		NotBefore: time.Date(2022, 6, 21, 13, 14, 15, 0, time.UTC),
	}
)

func TestNeedsRenewIn(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name          string
		expectedNames []string
		now           time.Time
		expected      time.Duration
	}{
		{
			name:          "all-valid",
			expectedNames: testNames,
			now:           time.Date(2023, 5, 21, 13, 14, 15, 0, time.UTC),
			expected:      17 * 24 * time.Hour,
		},
		{
			name:          "names-invalid",
			expectedNames: append(testNames, "test2"),
			now:           time.Date(2023, 5, 21, 13, 14, 15, 0, time.UTC),
			expected:      0,
		},
		{
			name:          "before-valid-date",
			expectedNames: testNames,
			now:           time.Date(2022, 6, 21, 13, 14, 14, 0, time.UTC),
			expected:      0,
		},
		{
			name:          "after-valid-date",
			expectedNames: testNames,
			now:           time.Date(2023, 6, 21, 13, 14, 15, 1, time.UTC),
			expected:      -14*24*time.Hour - 1*time.Nanosecond,
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual := NeedsRenewIn(&testCert, tcase.expectedNames, tcase.now)
			assert.Equal(t, tcase.expected, actual)
		})
	}
}
