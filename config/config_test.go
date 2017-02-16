package config

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	tmpfile, err := ioutil.TempFile("/tmp", "config_")
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())
	assert.Nil(t, New(tmpfile.Name(), ""))
}
