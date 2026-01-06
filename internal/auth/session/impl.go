package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const redisSessionPrefix = "SESSION"

type RedisRepo struct {
	redisClient *redis.Client
}

func NewRedisRepo(redisClient *redis.Client) Repo {
	return &RedisRepo{redisClient}
}

func (r *RedisRepo) Create(
	userId uuid.UUID,
	sessionLifetime Seconds,
) (*Model, error) {
	expiration := time.Second * time.Duration(sessionLifetime)

	session := Model{
		Id:        uuid.New(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(expiration),
		UserId:    userId,
	}

	sessionJson, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s:%s", redisSessionPrefix, session.Id.String())
	err = r.redisClient.Set(context.Background(), key, sessionJson, expiration).
		Err()
	if err != nil {
		return nil, err
	}

	zap.L().Debug(
		"Session created",
		zap.String("session_id", session.Id.String()),
		zap.String("user_id", userId.String()),
	)

	return &session, nil
}

func (r *RedisRepo) GetById(id uuid.UUID) (*Model, error) {
	key := fmt.Sprintf("%s:%s", redisSessionPrefix, id.String())
	sessionJson, err := r.redisClient.Get(context.Background(), key).Result()

	if err == redis.Nil {
		zap.L().
			Debug("Session not found", zap.String("session_id", id.String()))
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var session Model
	err = json.Unmarshal([]byte(sessionJson), &session)
	if err != nil {
		zap.L().Error("Failed to unmarshal session", zap.Error(err))
		return nil, err
	}
	zap.L().Debug("Session retrieved", zap.String("session_id", id.String()))

	return &session, nil
}

func (r *RedisRepo) DeleteById(id uuid.UUID) error {
	key := fmt.Sprintf("%s:%s", redisSessionPrefix, id.String())

	err := r.redisClient.Del(context.Background(), key).Err()
	if err != nil {
		zap.L().Error("Failed to delete session from Redis", zap.Error(err))
		return err
	}

	zap.L().Debug("Session deleted", zap.String("session_id", id.String()))

	return nil
}
