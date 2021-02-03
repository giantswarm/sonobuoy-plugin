package sonobuoy_plugin

import (
	"context"
	"fmt"
	"testing"

	"github.com/giantswarm/micrologger"
)

type TestLogger struct {
	logger micrologger.Logger
	t      *testing.T
}

func NewTestLogger(logger micrologger.Logger, t *testing.T) micrologger.Logger {
	return &TestLogger{
		logger: logger,
		t:      t,
	}
}

func (l *TestLogger) Debugf(ctx context.Context, format string, params ...interface{}) {
	l.logger.Debugf(ctx, "%s: %s", l.t.Name(), fmt.Sprintf(format, params...))
}

func (l *TestLogger) Errorf(ctx context.Context, err error, format string, params ...interface{}) {
	l.logger.Errorf(ctx, err, "%s: %s", l.t.Name(), fmt.Sprintf(format, params...))
}

func (l *TestLogger) Log(keyVals ...interface{}) {
	l.logger.Log(append(keyVals, "testName", l.t.Name())...)
}

func (l *TestLogger) LogCtx(ctx context.Context, keyVals ...interface{}) {
	l.logger.LogCtx(ctx, append(keyVals, "testName", l.t.Name())...)
}

func (l *TestLogger) With(keyVals ...interface{}) micrologger.Logger {
	return l.logger.With(append(keyVals, "testName", l.t.Name())...)
}
