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

var chatsCache = map[string]*tdlib.Chat{}

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
			panic(err)
		}
	})
	if err != nil {
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
		panic(err)
	}
}

func ForwardMessages(tg *tdlib.Client, username string, fromMessageID int64, to *tdlib.Chat) (int64, error) {
	if from, err := GetChat(tg, username); err == nil {
		if fromMessageID == 0 {
			fromMessageID = 1
		}
		if messages, err := tg.GetChatHistory(from.ID, fromMessageID, -99, 100, false); err == nil {
			if len(messages.Messages) > 0 {
				sort.SliceStable(messages.Messages, func(i, j int) bool {
					return messages.Messages[i].ID < messages.Messages[j].ID
				})
				var messageIDs []int64
				var mediaAlbumID tdlib.JSONInt64
				for _, msg := range messages.Messages {
					if msg.CanBeForwarded && fromMessageID < msg.ID {
						if len(messageIDs) == 0 {
							mediaAlbumID = msg.MediaAlbumID
						}
						if msg.MediaAlbumID != mediaAlbumID {
							break
						}
						messageIDs = append(messageIDs, msg.ID)
					}
				}
				if len(messageIDs) > 0 {
					log.Println(`fwd`, from.ID, `->`, to.ID, messageIDs, mediaAlbumID != 0)
					_, err := tg.ForwardMessages(to.ID, from.ID, messageIDs, false, false, mediaAlbumID != 0)
					if err == nil {
						return messageIDs[len(messageIDs)-1], nil
					} else {
						return 0, err
					}
				}
			}
		} else {
			return 0, err
		}
	} else {
		return 0, err
	}
	return fromMessageID, nil
}

func GetChat(tg *tdlib.Client, username string) (*tdlib.Chat, error) {
	if chat, found := chatsCache[username]; found {
		return chat, nil
	}
	if chat, err := tg.SearchPublicChat(username); err == nil {
		chatsCache[username] = chat
		return chat, err
	} else {
		return nil, err
	}
}

func Update(db *redis.Client, tg *tdlib.Client, chat *tdlib.Chat) {
	for username, fromMessageID := range GetChats(db) {
		if newFromMessageID, err := ForwardMessages(tg, username, fromMessageID, chat); err == nil {
			if newFromMessageID != fromMessageID {
				if err := db.HSet(redisChatsKey, username, strconv.FormatInt(newFromMessageID, 10)).Err(); err != nil {
					log.Println(err, username, fromMessageID, newFromMessageID)
					SentryHub(map[string]interface{}{`key`: username, `value`: fromMessageID, `new`: newFromMessageID}).CaptureException(err)
				}
			}
		} else {
			log.Println(err, username, fromMessageID)
			SentryHub(map[string]interface{}{`key`: username, `value`: fromMessageID}).CaptureException(err)
		}
	}
}

func GetChats(db *redis.Client) map[string]int64 {
	res := map[string]int64{}
	if data, err := db.HGetAll(redisChatsKey).Result(); err == nil {
		for username, fromMessageIDStr := range data {
			if fromMessageID, err := strconv.ParseInt(fromMessageIDStr, 10, 64); err == nil {
				res[username] = fromMessageID
			} else {
				log.Println(err, username, fromMessageIDStr)
				SentryHub(map[string]interface{}{`key`: username, `value`: fromMessageIDStr}).CaptureException(err)
			}
		}
	} else {
		panic(err)
	}
	return res
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

	chat, err := GetChat(tg, ChatUsername)
	if err != nil {
		panic(err)
	}

	for {
		Update(db, tg, chat)
		time.Sleep(time.Minute)
	}
}
