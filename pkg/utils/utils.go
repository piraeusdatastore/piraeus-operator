package utils

import (
	lapiconst "github.com/LINBIT/golinstor"
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
)

type SatelliteComType struct {
	Name string
	Port int32
}

func NewSatelliteComTypeForSslConfig(ssl *piraeusv1alpha1.JavaSslConfiguration) *SatelliteComType {
	if ssl != nil {
		return &SatelliteComType{Name: lapiconst.ValNetcomTypeSsl, Port: lapiconst.DfltStltPortSsl}
	}
	return &SatelliteComType{Name: lapiconst.ValNetcomTypePlain, Port: lapiconst.DfltStltPortPlain}
}
