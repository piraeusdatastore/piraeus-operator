package client

import (
	"fmt"

	lapi "github.com/LINBIT/golinstor/client"
	"github.com/go-logr/logr"

	. "github.com/piraeusdatastore/piraeus-operator/pkg/logconsts"
)

type logrAdapter struct {
	logr.Logger
}

var _ lapi.LeveledLogger = &logrAdapter{}

func Logr(l logr.Logger) lapi.Option {
	return lapi.Log(&logrAdapter{Logger: l})
}

func (l *logrAdapter) Errorf(s string, i ...interface{}) {
	l.Logger.Error(fmt.Errorf(s, i...), "error occurred")
}

func (l *logrAdapter) Warnf(s string, i ...interface{}) {
	l.Logger.V(0).Info(fmt.Sprintf(s, i...))
}

func (l *logrAdapter) Infof(s string, i ...interface{}) {
	l.Logger.V(INFO).Info(fmt.Sprintf(s, i...))
}

func (l *logrAdapter) Debugf(s string, i ...interface{}) {
	l.Logger.V(DEBUG).Info(fmt.Sprintf(s, i...))
}
