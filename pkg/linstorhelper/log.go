package linstorhelper

import (
	"fmt"

	lapi "github.com/LINBIT/golinstor/client"
	"github.com/go-logr/logr"
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
	l.Logger.V(0).Info(fmt.Sprintf(s, i...))
}

func (l *logrAdapter) Debugf(s string, i ...interface{}) {
	l.Logger.V(1).Info(fmt.Sprintf(s, i...))
}
