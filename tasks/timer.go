package tasks

import (
	"time"

	"github.com/InnovaCo/broforce/bus"
)

func init() {
	//registry("timer", bus.Task(&timer{}))
}

type Tact struct {
	Number int64 `json:"number"`
}

type timer struct {
	interval time.Duration
}

func (p *timer) handler(e bus.Event, ctx bus.Context) error {
	tact := Tact{}
	if err := bus.Encoder(e.Data, &tact, e.Coding); err != nil {
		return err
	}

	ctx.Log.Debugf("Tact: %d", tact.Number)
	return nil
}

func (p *timer) Run(eventBus *bus.EventsBus, ctx bus.Context) error {
	p.interval = time.Duration(ctx.Config.GetIntOr("interval", 1)) * time.Second
	eventBus.Subscribe(bus.TimerEvent, bus.Context{Func: p.handler, Name: "TimerHandler"})

	i := int64(0)
	e := bus.Event{
		Trace:   bus.NewUUID(),
		Subject: bus.TimerEvent,
		Coding:  bus.JsonCoding}

	tact := Tact{}
	for {
		tact.Number, i = i, i+1

		if err := bus.Coder(&e, tact); err != nil {
			ctx.Log.Error(err)
		}
		if err := eventBus.Publish(e); err != nil {
			ctx.Log.Error(err)
		}
		time.Sleep(p.interval)
	}
	ctx.Log.Debug("timeSensor Complete")
	return nil
}
