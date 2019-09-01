package main

import (
	"fmt"
	"github.com/Arman92/go-tdlib"
	"github.com/getsentry/sentry-go"
	"github.com/go-redis/redis"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

const tdlibDatabaseDir = "./tdlib-db"
const redisTdlibKey = "tdlib-db"
const redisChatsKey = "chats"

var Debug = os.Getenv(`DEBUG`) != ``
var ChatUsername = os.Getenv(`TG_USERNAME`)

func init() {
	tdlib.SetLogVerbosityLevel(1)
	if err := sentry.Init(sentry.ClientOptions{Debug: Debug}); err != nil {
		panic(err)
	}
}

func initTG() *tdlib.Client {
	tg, state, err := GetTGClient(os.Getenv(`TG_NUMBER`), os.Getenv(`TG_PASSWORD`), func() string {
		var code string
		if _, err := fmt.Scanln(&code); err == nil {
			return code
		} else {
			sentry.CaptureException(err)
			panic(err)
		}
	})
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
	}
	if state.GetAuthorizationStateEnum() != tdlib.AuthorizationStateReadyType {
		SentryHub(map[string]interface{}{`state`: state}).CaptureMessage(`Wrong telegram authorization state`)
		panic(state)
	}
	return tg
}

func initDB() *redis.Client {
	if db, err := GetRedis(os.Getenv(`REDIS_URL`)); err == nil {
		return db
	} else {
		sentry.CaptureException(err)
		panic(err)
	}
}

func ForwardMessages(tg *tdlib.Client, username string, fromMessageID int64, to *tdlib.Chat) (int64, error) {
	if chat, err := tg.SearchPublicChat(username); err == nil {
		if fromMessageID == 0 {
			fromMessageID = 1
		}
		if messages, err := tg.GetChatHistory(chat.ID, fromMessageID, -99, 100, false); err == nil {
			if len(messages.Messages) > 0 {
				sort.SliceStable(messages.Messages, func(i, j int) bool {
					return messages.Messages[i].ID < messages.Messages[j].ID
				})
				var messageIDs []int64
				for _, msg := range messages.Messages {
					if msg.CanBeForwarded && fromMessageID < msg.ID {
						messageIDs = append(messageIDs, msg.ID)
					}
				}
				if len(messageIDs) > 0 {
					log.Println(`fwd`, chat, `->`, to, messageIDs)
					_, err := tg.ForwardMessages(to.ID, chat.ID, messageIDs, false, false, false)
					if err == nil {
						return messages.Messages[len(messages.Messages)-1].ID, nil
					} else {
						return 0, err
					}
				}
			}
		} else {
			log.Println(err, chat, fromMessageID)
			SentryHub(map[string]interface{}{`chat`: chat, `offset`: fromMessageID}).CaptureException(err)
			return 0, err
		}
	} else {
		return 0, err
	}
	return fromMessageID, nil
}

func Update(db *redis.Client, tg *tdlib.Client, chat *tdlib.Chat)  {
	if data, err := db.HGetAll(redisChatsKey).Result(); err == nil {
		for username, fromMessageIDStr := range data {
			if fromMessageID, err := strconv.ParseInt(fromMessageIDStr, 10, 64); err == nil {
				if newFromMessageID, err := ForwardMessages(tg, username, fromMessageID, chat); err == nil {
					if newFromMessageID != 0 && newFromMessageID != fromMessageID {
						newFromMessageIDStr := strconv.FormatInt(newFromMessageID, 10)
						if err := db.HSet(redisChatsKey, username, newFromMessageIDStr).Err(); err != nil {
							log.Println(err, username, fromMessageIDStr, newFromMessageIDStr)
							SentryHub(map[string]interface{}{`key`: username, `value`: fromMessageIDStr, `new`: newFromMessageIDStr}).CaptureException(err)
						}
					}
				} else {
					log.Println(err, username, fromMessageIDStr)
					SentryHub(map[string]interface{}{`key`: username, `value`: fromMessageIDStr}).CaptureException(err)
				}
			} else {
				log.Println(err, username, fromMessageIDStr)
				SentryHub(map[string]interface{}{`key`: username, `value`: fromMessageIDStr}).CaptureException(err)
			}
		}
	} else {
		panic(err)
	}
}

func main() {
	db := initDB()
	initFileDatabase(db, redisTdlibKey, tdlibDatabaseDir)

	tg := initTG()

	defer func() {
		if err := sentry.Recover(); err != nil {
			log.Println(err)
		}

		defer db.Close()
		tg.Close()
		for {
			if state, err := tg.GetAuthorizationState(); err == nil {
				if state.GetAuthorizationStateEnum() == tdlib.AuthorizationStateClosedType {
					break
				}
			} else {
				sentry.CaptureException(err)
				log.Println(err)
				break
			}
		}
		finiFileDatabase(db, redisTdlibKey, tdlibDatabaseDir)
	}()

	chat, err := tg.SearchPublicChat(ChatUsername)
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(time.Minute)
		Update(db, tg, chat)
	}

}
