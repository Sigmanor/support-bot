package bot

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/samarets/support-bot/internal/log"
)

const (
	startCommand   = "start"
	connectCommand = "connect"
	breakCommand   = "break"
	cancelCommand  = "cancel"
	getID          = "getid"

	setGroup   = "set_group"
	addSupport = "add_support"
	event      = "event"
)

func (b *bot) StartCommand(update tgbotapi.Update, userState state) {
	if !update.FromChat().IsPrivate() {
		return
	}

	var message string
	switch userState {
	case queueState:
		message = b.tl.GetMessage(b.db.languageDB().get(update.SentFrom().ID), "queue_start")
	case roomState:
		message = b.tl.GetMessage(b.db.languageDB().get(update.SentFrom().ID), "room_start")
	default:
		message = b.tl.GetMessage(
			b.db.languageDB().get(update.SentFrom().ID), "hello", map[string]interface{}{
				"Name": update.SentFrom().FirstName,
			},
		)
	}

	msg := tgbotapi.NewMessage(
		update.Message.Chat.ID,
		message,
	)

	_, err := b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
	}
}

func (b *bot) ConnectCommand(update tgbotapi.Update) {
	pingUser := update.SentFrom()

	userTg, err := b.db.queueDB().get(pingUser.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	if userTg != nil {
		return
	}

	chatID, err := b.db.roomsDB().get(pingUser.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	if chatID != nil {
		return
	}

	user, err := b.db.queueDB().getFirst()
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	if user == nil {
		return
	}

	err = b.db.roomsDB().set(user.ID, pingUser.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	err = b.db.roomsDB().set(pingUser.ID, user.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	err = b.db.queueDB().delete(user.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg := tgbotapi.NewMessage(
		user.ID,
		b.tl.GetMessage(b.db.languageDB().get(user.ID), "operator_connected"),
	)
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg = tgbotapi.NewMessage(
		pingUser.ID,
		b.tl.GetMessage(
			b.db.languageDB().get(pingUser.ID), "user_connected", map[string]interface{}{
				"Name": user.FirstName + " " + user.LastName,
				"ID":   user.ID,
			},
		),
	)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	bufferMessages, err := b.db.bufferDB().get(user.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	for _, message := range bufferMessages {
		msg := tgbotapi.NewCopyMessage(
			pingUser.ID,
			message.Chat.ID,
			message.MessageID,
		)
		rMsg, err := b.bot.Send(msg)
		if err != nil {
			log.Error().Err(err).Send()
			return
		}

		err = b.db.messagesIDsDB().set(message.MessageID, rMsg.MessageID)
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}

		err = b.db.messagesIDsDB().set(rMsg.MessageID, message.MessageID)
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}
	}

	err = b.db.bufferDB().delete(user.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}

func (b *bot) BreakCommand(update tgbotapi.Update) {
	whomBreak, err := b.db.roomsDB().get(update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	if whomBreak == nil {
		return
	}

	err = b.db.roomsDB().delete(*whomBreak)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	err = b.db.roomsDB().delete(update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg := tgbotapi.NewMessage(*whomBreak, "🤖 Розмову з оператором було завершено")
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg = tgbotapi.NewMessage(update.Message.Chat.ID, "🤖 Ви завершили розмову з користувачем")
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}

func (b *bot) CancelCommand(update tgbotapi.Update) {
	userTg, err := b.db.queueDB().get(update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	if userTg == nil {
		return
	}

	err = b.db.queueDB().delete(update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	err = b.db.bufferDB().delete(update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🤖 Ви були видалені з черги")
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}

func (b *bot) GetID(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("🤖 %d", update.SentFrom().ID))
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}

func (b *bot) SetGroup(update tgbotapi.Update) {
	user := update.SentFrom()
	if user.ID != b.adminID {
		return
	}

	err := b.db.groupDB().set(update.FromChat().ID)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg := tgbotapi.NewMessage(
		update.FromChat().ID,
		b.tl.GetMessage(b.db.languageDB().get(user.ID), "channel_saved"),
	)
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}

func (b *bot) AddSupport(update tgbotapi.Update, adminID int64) {
	if update.SentFrom().ID != adminID {
		return
	}

	var userID int64
	if update.Message.ReplyToMessage != nil && !update.Message.ReplyToMessage.From.IsBot {
		if update.Message.ReplyToMessage.From.ID == adminID {
			return
		}
		userID = update.Message.ReplyToMessage.From.ID
	} else {
		argumentID, err := strconv.ParseInt(update.Message.CommandArguments(), 10, 64)
		if err != nil {
			msg := tgbotapi.NewMessage(
				update.Message.Chat.ID,
				b.tl.GetMessage(b.db.languageDB().get(update.SentFrom().ID), "add_support_fail"),
			)
			msg.ReplyToMessageID = update.Message.MessageID
			msg.ParseMode = tgbotapi.ModeMarkdown
			_, err := b.bot.Send(msg)
			if err != nil {
				log.Error().Err(err).Send()
				return
			}
			return
		}
		if argumentID == adminID {
			return
		}
		userID = argumentID
	}

	err := b.db.supportDB().set(userID, true)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	msg := tgbotapi.NewMessage(
		update.Message.Chat.ID,
		b.tl.GetMessage(
			b.db.languageDB().get(update.SentFrom().ID), "add_support_success", map[string]interface{}{
				"UserID": userID,
			},
		),
	)
	msg.ReplyToMessageID = update.Message.MessageID
	_, err = b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}

func (b *bot) Event(update tgbotapi.Update) {
	groupID := b.db.groupDB().get()

	msg := tgbotapi.NewMessage(
		groupID,
		"event",
	)
	_, err := b.bot.Send(msg)
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
}
