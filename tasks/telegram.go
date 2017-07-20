package tasks

import (
	"strings"

	"gopkg.in/telegram-bot-api.v4"

	"github.com/mhanygin/broforce/bus"
)

func init() {
	registry("telegramSensor", bus.Task(&sensorTelegram{}))
}

//config section
//
//telegramSensor:
//  allowed_users: ["user_1", "user_2", ...]
//  token: TOKEN
//  defaultChanID
//

type sensorTelegram struct {
	client        *tgbotapi.BotAPI
	defaultChatID int
}

func (p *sensorTelegram) validator(msg *tgbotapi.Message, allowed []string) bool {
	for _, username := range allowed {
		if strings.Compare(msg.From.UserName, username) == 0 {
			return true
		}
	}
	return false
}

func (p *sensorTelegram) messageEvent(msg tgbotapi.Update, ctx *bus.Context) error {
	uuid := bus.NewUUID()
	if event, err := bus.NewEventWithData(uuid, bus.TelegramMsgEvent, bus.JsonCoding, msg.Message); err != nil {
		return err
	} else {
		ctx.Log.Debugf("Push: %s", uuid)
		return ctx.Bus.Publish(*event)
	}
}

func (p *sensorTelegram) postMessage(e bus.Event, ctx bus.Context) error {
	return nil
}

func (p *sensorTelegram) Run(ctx bus.Context) error {
	var err error
	p.client, err = tgbotapi.NewBotAPI(ctx.Config.GetStringOr("token", ""))
	if err != nil {
		return err
	}
	p.defaultChatID = ctx.Config.GetIntOr("chat_id", 0)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := p.client.GetUpdatesChan(u)

	allowedUsers := ctx.Config.GetArrayString("allowed_users")

	for update := range updates {
		if (update.Message == nil) || !p.validator(update.Message, allowedUsers) {
			continue
		}
		p.messageEvent(update, &ctx)
	}
	return nil
}
