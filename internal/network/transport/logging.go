package transport

import (
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func silentWakuLogger() *zap.Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(io.Discard),
		zapcore.DebugLevel,
	)
	return zap.New(core)
}
