package logger

import (
	"os"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/mhanygin/broforce/config"
	"github.com/mhanygin/broforce/logger/fluent"
)

var (
	once sync.Once
	Log  *logrus.Logger
)

func New(cfg config.ConfigData) *logrus.Logger {
	once.Do(func() {
		Log = logrus.StandardLogger()
		Log.Formatter = &logrus.TextFormatter{TimestampFormat: time.RFC3339, FullTimestamp: true}

		if cfg.Exist("file") {
			f, err := os.OpenFile(cfg.GetStringOr("file.name", "broforce.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				panic(err)
			}
			logrus.SetOutput(f)
			if lvl, err := logrus.ParseLevel(cfg.GetStringOr("file.level", "info")); err == nil {
				logrus.SetLevel(lvl)
			}
		}

		if cfg.Exist("fluentd") {
			if hook, err := logrus_fluent.New(cfg.GetStringOr("fluentd.host", "localhost"), cfg.GetIntOr("fluentd.port", 24224)); err == nil {
				levels := []logrus.Level{}
				for _, lvl := range cfg.GetArrayString("fluentd.levels") {
					if l, err := logrus.ParseLevel(lvl); err == nil {
						levels = append(levels, l)
					}
				}
				Log.Debugf("fluentd levels: %v", levels)

				hook.SetLevels(levels)
				hook.SetTag(cfg.GetStringOr("tag", "broforce"))
				logrus.AddHook(hook)
			} else {
				Log.Errorf("fluentd: %v", err)
			}
		}
	})
	return Log
}

func Logger4Handler(name string, trace string) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"handler": name,
		"trace":   trace,
	})
}
