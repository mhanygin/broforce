package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/mhanygin/broforce/bus"
	"github.com/mhanygin/broforce/config"
	"github.com/mhanygin/broforce/logger"
	"github.com/mhanygin/broforce/tasks"
)

var Version = ""

func main() {
	cfgPath := kingpin.Flag("config", "Path to config.yml file.").Default("config.yml").String()
	show := kingpin.Flag("show", "Show all task names.").Bool()
	allow := kingpin.Flag("allow", "list of allowed tasks").Default(tasks.GetPoolString()).String()

	kingpin.Version(Version)
	kingpin.Parse()

	if *show {
		fmt.Println("name bus adapters:")
		for _, n := range bus.GetNameAdapters() {
			fmt.Println(" - ", n)
		}
		fmt.Println("task names:")
		for n := range tasks.GetPool() {
			fmt.Println(" - ", n)
		}
		return
	}

	if _, err := os.Stat(*cfgPath); os.IsNotExist(err) {
		fmt.Errorf("%v", err)
		return
	}
	allowTasks := fmt.Sprintf(",%s,", *allow)
	c := config.New(*cfgPath, config.YAMLAdapter)
	if c == nil {
		fmt.Println("Error: config not create")
		return
	}
	logger.New(c.Get("logger"))

	logger.Log.Debugf("Config for bus: %v", c.Get("bus"))

	b := bus.New(c.Get("bus"))
	for n, s := range tasks.GetPool() {
		if strings.Index(allowTasks, fmt.Sprintf(",%s,", n)) != -1 {

			logger.Log.Debugf("Config for %s: %v", n, c.Get(n))

			go bus.SafeRun(s.Run, bus.SafeParams{Retry: 0, Delay: 0})(
				bus.Context{
					Name:   n,
					Config: c.Get(n),
					Log:    logger.Logger4Handler(n, ""),
					Bus:    b})
		}
	}

	runtime.Goexit()
}
