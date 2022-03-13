package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

const (
	selectShopAction  = "select-shop"
	selectDrinkAction = "select-drink"
	confirmAction     = "confirm-to-buy"
	cancelAction      = "cancel"
)

func main() {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		panic("SLACK_BOT_TOKEN is not set")
	}
	client := slack.New(token)

	http.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		parsedEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		switch parsedEvent.Type {
		case slackevents.URLVerification:
			res := new(slackevents.ChallengeResponse)
			if err := json.Unmarshal(body, res); err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err := w.Write([]byte(res.Challenge)); err != nil {
				log.Panicln(err)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
		case slackevents.CallbackEvent:
			innerEvent := parsedEvent.InnerEvent
			switch event := innerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				msg := strings.Split(event.Text, " ")
				if len(msg) < 2 {
					log.Panicln("Number of messages is not enough")
					w.WriteHeader(http.StatusBadRequest)

					return
				}

				command := msg[1]
				switch command {
				case "buy":
					// お店を選ぶ
					text := slack.NewTextBlockObject(slack.PlainTextType, "Which store do you buy from?", false, false)
					textSection := slack.NewSectionBlock(text, nil, nil)

					// お店の選択肢を作成する
					shops := []string{"Starbucks", "Veloce", "Doutor"}
					options := make([]*slack.OptionBlockObject, 0, len(shops))
					for _, drink := range shops {
						optionText := slack.NewTextBlockObject(slack.PlainTextType, drink, false, false)
						options = append(options, slack.NewOptionBlockObject(drink, optionText, nil))
					}
					placeholder := slack.NewTextBlockObject(slack.PlainTextType, "Shop", false, false)
					selectMenu := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, placeholder, "", options...)

					actionBlock := slack.NewActionBlock(selectShopAction, selectMenu)

					blocks := slack.MsgOptionBlocks(textSection, actionBlock)

					if _, _, err := client.PostMessage(event.Channel, blocks); err != nil {
						log.Println(err)
						w.WriteHeader(http.StatusInternalServerError)

						return
					}
				}
			}
		}
	})

	http.HandleFunc("/slack/actions", func(w http.ResponseWriter, r *http.Request) {
		payload := new(slack.InteractionCallback)
		if err := json.Unmarshal([]byte(r.FormValue("payload")), payload); err != nil {
			log.Panicln(err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		switch payload.Type {
		case slack.InteractionTypeBlockActions:
			if len(payload.ActionCallback.BlockActions) == 0 {
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			action := payload.ActionCallback.BlockActions[0]
			switch action.BlockID {
			case selectShopAction:
				// 飲み物を選ぶ
				text := slack.NewTextBlockObject(slack.PlainTextType, "What do you need?", false, false)
				textSection := slack.NewSectionBlock(text, nil, nil)

				// 飲み物の選択肢を作成する
				drinks := []string{"coffee", "latte", "juice"}
				options := make([]*slack.OptionBlockObject, 0, len(drinks))
				for _, drink := range drinks {
					optionText := slack.NewTextBlockObject(slack.PlainTextType, drink, false, false)
					options = append(options, slack.NewOptionBlockObject(drink, optionText, nil))
				}
				placeholder := slack.NewTextBlockObject(slack.PlainTextType, "Drink", false, false)
				selectMenu := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, placeholder, "", options...)

				actionBlock := slack.NewActionBlock(selectDrinkAction, selectMenu)

				blocks := slack.MsgOptionBlocks(textSection, actionBlock)

				replaceOriginal := slack.MsgOptionReplaceOriginal(payload.ResponseURL)
				if _, _, _, err := client.SendMessage(payload.ResponseURL, replaceOriginal, blocks); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)

					return
				}
			case selectDrinkAction:
				// 確認ボタンを送付する
				drink := action.SelectedOption.Value

				text := slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("Can I buy `%s` from `%s` ?", drink, ""), false, false)
				textSection := slack.NewSectionBlock(text, nil, nil)

				// Okボタン
				confirmButtonText := slack.NewTextBlockObject(slack.PlainTextType, "Ok", false, false)
				confirmButton := slack.NewButtonBlockElement(confirmAction, drink, confirmButtonText)
				confirmButton.WithStyle(slack.StylePrimary)

				// キャンセルボタン
				cancelButtonText := slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false)
				cancelButton := slack.NewButtonBlockElement(cancelAction, "cancel", cancelButtonText)
				cancelButton.WithStyle(slack.StyleDanger)

				actionBlock := slack.NewActionBlock(confirmAction, confirmButton, cancelButton)
				blocks := slack.MsgOptionBlocks(textSection, actionBlock)

				replaceOriginal := slack.MsgOptionReplaceOriginal(payload.ResponseURL)
				if _, _, _, err := client.SendMessage(payload.Channel.ID, replaceOriginal, blocks); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)

					return
				}
			}
		case cancelAction:
		}
	})

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
