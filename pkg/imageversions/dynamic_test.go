package imageversions_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	lclient "github.com/LINBIT/golinstor/client"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/imageversions"
)

func TestFromConfigMap(t *testing.T) {
	fakeclient := fake.NewFakeClient(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "image-config",
			Namespace: "test",
		},
		Data: map[string]string{
			"0_image.yaml": `
base: example.com/base
components:
  linstor-satellite:
    image: satellite-image
    tag: v1.2.3
  other-image:
    image: other
    tag: v0.1.2
`,
			"1_image.yaml": `
base: example.com/extra
components:
  linstor-controller:
    image: controller-image
    tag: v2.0.0
  other-image:
    image: another
    tag: v2.1.0
  multi:
    image: fallback
    tag: v10.11.12
    match:
      - image: abcd
        osImage: efgh
        precompiled: true
      - image: "1234"
        osImage: "9876"
`,
		},
	})

	actual, err := imageversions.FromConfigMap(context.Background(), fakeclient, types.NamespacedName{Namespace: "test", Name: "image-config"})
	assert.NoError(t, err)
	assert.Equal(t, imageversions.Configs{
		{
			Base: "example.com/base",
			Components: map[string]imageversions.ComponentConfig{
				"linstor-satellite": {
					Image: "satellite-image",
					Tag:   "v1.2.3",
				},
				"other-image": {
					Image: "other",
					Tag:   "v0.1.2",
				},
			},
		},
		{
			Base: "example.com/extra",
			Components: map[string]imageversions.ComponentConfig{
				"linstor-controller": {
					Image: "controller-image",
					Tag:   "v2.0.0",
				},
				"other-image": {
					Image: "another",
					Tag:   "v2.1.0",
				},
				"multi": {
					Image: "fallback",
					Tag:   "v10.11.12",
					Match: []imageversions.OsMatch{
						{Image: "abcd", OsImage: "efgh", Precompiled: true},
						{Image: "1234", OsImage: "9876", Precompiled: false},
					},
				},
			},
		},
	}, actual)
}

func TestSetFromExternalCluster(t *testing.T) {
	configs := imageversions.Configs{
		{
			Base: "example.com/base",
			Components: map[string]imageversions.ComponentConfig{
				"linstor-satellite": {
					Image: "satellite-image",
					Tag:   "v1.2.3",
				},
				"linstor-controller": {
					Image: "controller-image",
					Tag:   "v2.0.0",
				},
			},
		},
		{
			Base: "example.com/extra",
			Components: map[string]imageversions.ComponentConfig{
				"linstor-satellite": {
					Image: "other-satellite-image",
					Tag:   "other-tag",
				},
				"linstor-controller": {
					Image: "other-controller-image",
					Tag:   "v2.0.0-other",
				},
			},
		},
	}

	err := imageversions.SetFromExternalCluster(context.Background(), &lclient.Client{
		Controller: &FakeVersionReporter{
			Error: fmt.Errorf("error"),
		},
	}, configs)
	assert.Error(t, err)

	err = imageversions.SetFromExternalCluster(context.Background(), &lclient.Client{
		Controller: &FakeVersionReporter{
			ControllerVersion: lclient.ControllerVersion{Version: "10.11.12"},
		},
	}, configs)
	assert.NoError(t, err)
	assert.Equal(t, imageversions.Configs{
		{
			Base: "example.com/base",
			Components: map[string]imageversions.ComponentConfig{
				"linstor-satellite": {
					Image: "satellite-image",
					Tag:   "v10.11.12",
				},
				"linstor-controller": {
					Image: "controller-image",
					Tag:   "v2.0.0",
				},
			},
		},
		{
			Base: "example.com/extra",
			Components: map[string]imageversions.ComponentConfig{
				"linstor-satellite": {
					Image: "other-satellite-image",
					Tag:   "other-tag",
				},
				"linstor-controller": {
					Image: "other-controller-image",
					Tag:   "v2.0.0-other",
				},
			},
		},
	}, configs)
}

type FakeVersionReporter struct {
	lclient.ControllerVersion
	Error error
}

var _ lclient.ControllerProvider = &FakeVersionReporter{}

func (f *FakeVersionReporter) GetVersion(ctx context.Context, opts ...*lclient.ListOpts) (lclient.ControllerVersion, error) {
	return f.ControllerVersion, f.Error
}

func (f *FakeVersionReporter) GetConfig(ctx context.Context, opts ...*lclient.ListOpts) (lclient.ControllerConfig, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) Modify(ctx context.Context, props lclient.GenericPropsModify) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetProps(ctx context.Context, opts ...*lclient.ListOpts) (lclient.ControllerProps, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) DeleteProp(ctx context.Context, prop string) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetErrorReports(ctx context.Context, opts ...*lclient.ListOpts) ([]lclient.ErrorReport, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) DeleteErrorReports(ctx context.Context, del lclient.ErrorReportDelete) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetErrorReportsSince(ctx context.Context, since time.Time, opts ...*lclient.ListOpts) ([]lclient.ErrorReport, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetErrorReport(ctx context.Context, id string, opts ...*lclient.ListOpts) (lclient.ErrorReport, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) CreateSOSReport(ctx context.Context, opts ...*lclient.ListOpts) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) DownloadSOSReport(ctx context.Context, writer io.WriteCloser, opts ...*lclient.ListOpts) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetSatelliteConfig(ctx context.Context, node string) (lclient.SatelliteConfig, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) ModifySatelliteConfig(ctx context.Context, node string, cfg lclient.SatelliteConfig) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetPropsInfos(ctx context.Context, opts ...*lclient.ListOpts) ([]lclient.PropsInfo, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetPropsInfosAll(ctx context.Context, opts ...*lclient.ListOpts) ([]lclient.PropsInfo, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetExternalFiles(ctx context.Context, opts ...*lclient.ListOpts) ([]lclient.ExternalFile, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) GetExternalFile(ctx context.Context, name string) (lclient.ExternalFile, error) {
	panic("unimplemented")
}

func (f *FakeVersionReporter) ModifyExternalFile(ctx context.Context, name string, file lclient.ExternalFile) error {
	panic("unimplemented")
}

func (f *FakeVersionReporter) DeleteExternalFile(ctx context.Context, name string) error {
	panic("unimplemented")
}
