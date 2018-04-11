package log

import (
	"github.com/sirupsen/logrus"
)

// Logger is a facade interface compatible with logrus.Logger but with limited scope of logging.
// It exists to decouple plugin implementations from particular log implementation but also to only allow
// reporting actionable problems and corner cases such as panic (or in other words to avoid logging as program flow analysis / debugging).
// If needed this can be extended by adding other levels such as Panic or Fatal (both are exiting, former go-routine or the program if unwinding reaches
// the top of the goroutine stack, whereas latter terminates the program immediately)
type Logger interface {
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

// ConfigureLogrus defines global formatting, level and fields used while logging
func ConfigureLogrus(pluginName string) *logrus.Entry {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.WarnLevel)
	log := logrus.WithField("ike-plugins", pluginName)
	return log
}
