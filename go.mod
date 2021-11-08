module github.com/piraeusdatastore/piraeus-operator

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/LINBIT/golinstor v0.37.1
	github.com/coreos/prometheus-operator v0.41.1
	github.com/linbit/k8s-await-election v0.2.3
	github.com/operator-framework/operator-sdk v0.19.4
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/pflag v1.0.5
	gopkg.in/ini.v1 v1.51.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.9.2
)

replace (
	github.com/satori/go.uuid => github.com/satori/go.uuid v1.2.0 // Required by golinstor
	k8s.io/client-go => k8s.io/client-go v0.21.2 // Required by prometheus-operator
)
