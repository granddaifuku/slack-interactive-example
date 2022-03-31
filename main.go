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
	selectDrinkAction = "select-drink"
	confirmAction     = "confirm-to-buy"
	okAction          = "ok"
	cancelAction      = "cancel"
)

func main() {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		panic("SLACK_BOT_TOKEN is not set")
	}
	client := slack.New(token)

	// お店の商品一覧
	shops := map[string][]string{
		"Starbucks": {"Caramel Frappucino", "Java Chip Frappuccino", "White Chocolate Mocha"},
		"Veloce":    {"Blend Coffee", "Hot Chocolate", "Latte"},
	}

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
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
		case slackevents.CallbackEvent:
			innerEvent := parsedEvent.InnerEvent
			switch event := innerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				msg := strings.Split(event.Text, " ")
				if len(msg) < 3 {
					log.Println("Number of messages is not enough")
					w.WriteHeader(http.StatusBadRequest)

					return
				}

				command := msg[1]
				shop := msg[2]
				switch command {
				case "buy":
					// 飲み物を選ぶ
					text := slack.NewTextBlockObject(slack.PlainTextType, "What do you need?", false, false)
					textSection := slack.NewSectionBlock(text, nil, nil)

					// 飲み物の選択肢を作成する
					drinks, ok := shops[shop]
					// リストにないお店だった
					if !ok {
						log.Println("We cannot buy drink from " + shop)
						w.WriteHeader(http.StatusBadRequest)
					}
					options := make([]*slack.OptionBlockObject, 0, len(drinks))
					for _, drink := range drinks {
						optionText := slack.NewTextBlockObject(slack.PlainTextType, drink, false, false)
						options = append(options, slack.NewOptionBlockObject(drink, optionText, nil))
					}
					placeholder := slack.NewTextBlockObject(slack.PlainTextType, "Drink", false, false)
					selectMenu := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, placeholder, "", options...)

					blockID := fmt.Sprintf("%s=%s", selectDrinkAction, shop)
					actionBlock := slack.NewActionBlock(blockID, selectMenu)

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
			log.Println(err)
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
			blockID := action.BlockID
			actions := strings.Split(blockID, "=") // = で block ID を分割する
			id := actions[0]                       // actions[0] に ID が入っている
			switch id {
			case selectDrinkAction:
				// 確認ボタンを送付する
				drink := action.SelectedOption.Value

				shop := actions[1] // actions[1] にお店が入っている
				text := slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("Can I buy `%s` from `%s` ?", drink, shop), false, false)
				textSection := slack.NewSectionBlock(text, nil, nil)

				// Okボタン
				okButtonText := slack.NewTextBlockObject(slack.PlainTextType, "Ok", false, false)
				okButton := slack.NewButtonBlockElement(okAction, drink, okButtonText)
				okButton.WithStyle(slack.StylePrimary)

				// キャンセルボタン
				cancelButtonText := slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false)
				cancelButton := slack.NewButtonBlockElement(cancelAction, "cancel", cancelButtonText)
				cancelButton.WithStyle(slack.StyleDanger)

				actionBlock := slack.NewActionBlock(confirmAction, okButton, cancelButton)
				blocks := slack.MsgOptionBlocks(textSection, actionBlock)

				replaceOriginal := slack.MsgOptionReplaceOriginal(payload.ResponseURL)
				if _, _, _, err := client.SendMessage(payload.Channel.ID, replaceOriginal, blocks); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)

					return
				}
			}
		case okAction:
			// 何か処理をする：ここでは購入の処理など
		case cancelAction:
			// 再度選択肢を提示する
		}
	})

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
