package logger

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InnovaCo/broforce/config"
)

func TestLogger4Handler(t *testing.T) {
	log := Logger4Handler("test", "trace")

	assert.Equal(t, log.Data["handler"], "test")
	assert.Equal(t, log.Data["trace"], "trace")
}

func TestNew(t *testing.T) {
	tmpfile, err := ioutil.TempFile("/tmp", "config_")
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())
	cfg := config.New(tmpfile.Name(), config.YAMLAdapter)
	if cfg == nil {
		t.Error("Config is nil")
		t.Fail()
	}
	assert.NotNil(t, New(cfg.Get("logger")))
}
