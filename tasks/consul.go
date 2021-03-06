package tasks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/hashicorp/consul/api"

	"github.com/mhanygin/broforce/bus"
	"github.com/mhanygin/broforce/config"
)

func init() {
	registry("consulSensor", bus.Task(&consulSensor{}))
	registry("outdated", bus.Task(&outdatedConsul{}))
}

//config section
//
//consulSensor:
//  consul:
//    - server1
//    - server2
//  key-outdate: "path/to/key/outdated"
//  key-data: "path/to/key/data"
//

const (
	dataPrefix     = "services/data"
	outdatedPrefix = "services/outdated"
	loopInterval   = 10
)

type outdatedEvent struct {
	EndOfLife int64  `json:"endOfLife"`
	Key       string `json:"key"`
	Address   string `json:"address"`
}

type consulSensor struct {
	clientsPool map[string]*api.Client
}

func (p *consulSensor) prepareConfig(cfg config.ConfigData) []*api.Config {
	dc := make([]*api.Config, 0)
	for _, address := range cfg.GetArrayString("consul") {
		c := api.DefaultConfig()
		c.Address = address
		dc = append(dc, c)
	}
	return dc
}

func (p *consulSensor) Run(ctx bus.Context) error {
	p.clientsPool = make(map[string]*api.Client)

	for _, c := range p.prepareConfig(ctx.Config) {
		client, err := api.NewClient(c)
		if err != nil {
			ctx.Log.Error(err)
			continue
		}
		p.clientsPool[c.Address] = client
	}
	for {
		for address, client := range p.clientsPool {
			kv := client.KV()
			pairs, _, err := kv.List(outdatedPrefix, nil)
			if err != nil {
				ctx.Log.Error(err)
				continue
			}
			for _, key := range pairs {
				outdated := outdatedEvent{EndOfLife: -1}
				if err := json.Unmarshal(key.Value, &outdated); err != nil {
					ctx.Log.Error(err)
				}
				if outdated.EndOfLife == -1 {
					continue
				}
				if outdated.EndOfLife < time.Now().UnixNano()/int64(time.Millisecond) {
					ctx.Log.Debugf("%s KV: %v=%v, outdated",
						address,
						string(key.Key),
						string(key.Value))

					outdated.Key = strings.Replace(key.Key, fmt.Sprintf("%s/", outdatedPrefix), "", 1)
					outdated.Address = address
					if event, err := bus.NewEventWithData(bus.NewUUID(), bus.OutdatedEvent, bus.JsonCoding, outdated); err != nil {
						ctx.Log.Error(err)
					} else if err := ctx.Bus.Publish(*event); err != nil {
						ctx.Log.Error(err)
					}
				} else {
					ctx.Log.Debugf("%s KV: %v=%v, delta: %v",
						address,
						string(key.Key),
						string(key.Value),
						outdated.EndOfLife-time.Now().UnixNano()/int64(time.Millisecond))
				}
			}
		}
		time.Sleep(loopInterval * time.Second)
	}
	ctx.Log.Debug("consulSensor Complete")
	return nil
}

type outdatedConsul struct {
}

func (p *outdatedConsul) handler(e bus.Event, ctx bus.Context) error {
	event := outdatedEvent{}
	if err := e.Unmarshal(&event); err != nil {
		return err
	}

	ctx.Log.Debug(event)

	conf := api.DefaultConfig()
	conf.Address = event.Address
	client, err := api.NewClient(conf)
	if err != nil {
		return err
	}
	kv := client.KV()
	pairs, _, err := kv.List(fmt.Sprintf("%s/%s/", dataPrefix, event.Key), nil)
	if err != nil {
		return err
	}

	if len(pairs) == 0 {
		ctx.Log.Infof("%s: key %s empty, delete key: %s",
			conf.Address,
			fmt.Sprintf("%s/%s/", dataPrefix, event.Key),
			fmt.Sprintf("%s/%s", outdatedPrefix, event.Key))

		if _, err := kv.Delete(fmt.Sprintf("%s/%s", outdatedPrefix, event.Key), nil); err != nil {
			return err
		}
		return nil
	}

	serveEvent := bus.NewEvent(e.Trace, bus.ServeCmdWithDataEvent, bus.JsonCoding)

	for _, key := range pairs {
		ctx.Log.Debugf("%s purge: %v=%v", conf.Address, string(key.Key), string(key.Value))
		g, err := gabs.ParseJSON(key.Value)
		if err != nil {
			ctx.Log.Error(err)
			continue
		}
		g.Set("true", "purge")
		plugin := strings.Split(key.Key, "/")

		params := serveParams{
			Vars:     map[string]string{"purge": "true"},
			Plugin:   plugin[len(plugin)-1],
			Manifest: g.Bytes()}

		if err := serveEvent.Marshal(params); err != nil {
			ctx.Log.Error(err)
			continue
		}
		if err := ctx.Bus.Publish(*serveEvent); err != nil {
			ctx.Log.Error(err)
		}
	}
	return nil
}

func (p *outdatedConsul) Run(ctx bus.Context) error {
	ctx.Bus.Subscribe(bus.OutdatedEvent, bus.Context{
		Func:   p.handler,
		Name:   "OutdatedHandler",
		Bus:    ctx.Bus,
		Config: ctx.Config})
	return nil
}
