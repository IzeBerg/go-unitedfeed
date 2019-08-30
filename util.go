package main

import (
	"github.com/Arman92/go-tdlib"
	"github.com/getsentry/sentry-go"
	"github.com/go-redis/redis"
	"os"
)

func SentryHub(extras map[string]interface{}) *sentry.Hub {
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		if extras != nil {
			scope.SetExtras(extras)
		}
	})
	return hub
}

func GetTGClient(phone, password string, getCode func() string) (*tdlib.Client, tdlib.AuthorizationState, error) {
	client := tdlib.NewClient(tdlib.Config{
		APIID:                  os.Getenv(`TG_API_ID`),
		APIHash:                os.Getenv(`TG_API_HASH`),
		SystemLanguageCode:     "en",
		DeviceModel:            "Server",
		SystemVersion:          "1.0.0",
		ApplicationVersion:     "1.0.0",
		DatabaseDirectory:      tdlibDatabaseDir,
		EnableStorageOptimizer: true,
		//UseTestDataCenter:  Debug,
	})

	for {
		if currentState, err := client.Authorize(); err == nil {
			if _, err := client.SetDatabaseEncryptionKey(nil); err != nil {
				return client, currentState, err
			}
			switch currentState.GetAuthorizationStateEnum() {
			case tdlib.AuthorizationStateWaitPhoneNumberType:
				if _, err := client.SendPhoneNumber(phone); err != nil {
					return client, currentState, err
				}
				break
			case tdlib.AuthorizationStateWaitCodeType:
				if _, err := client.SendAuthCode(getCode()); err != nil {
					return client, currentState, err
				}
				break
			case tdlib.AuthorizationStateWaitPasswordType:
				if _, err := client.SendAuthPassword(password); err != nil {
					return client, currentState, err
				}
				break
			case tdlib.AuthorizationStateWaitEncryptionKeyType:
				break
			default:
				return client, currentState, nil
			}
		} else {
			return client, currentState, err
		}
	}
}

func GetRedis(connectURL string) (*redis.Client, error) {
	if opts, err := redis.ParseURL(connectURL); err == nil {
		redisDB := redis.NewClient(opts)
		if err := redisDB.Ping().Err(); err != nil {
			return nil, err
		}
		return redisDB, nil
	} else {
		return nil, err
	}
}
