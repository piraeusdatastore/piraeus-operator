module github.com/piraeusdatastore/piraeus-operator

go 1.13

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/LINBIT/golinstor v0.33.0
	github.com/linbit/k8s-await-election v0.2.3
	github.com/operator-framework/operator-sdk v0.19.4
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/pflag v1.0.5
	gopkg.in/ini.v1 v1.51.0
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.0
)

// Pinned to kubernetes-1.16.2
replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Required by prometheus-operator
)
