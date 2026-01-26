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
	userID uuid.UUID,
	sessionLifetime Seconds,
) (*Model, error) {
	expiration := time.Second * time.Duration(sessionLifetime)

	session := Model{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(expiration),
		UserID:    userID,
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s:%s", redisSessionPrefix, session.ID.String())
	err = r.redisClient.Set(context.Background(), key, sessionJSON, expiration).
		Err()
	if err != nil {
		return nil, err
	}

	zap.L().Debug(
		"Session created",
		zap.String("session_id", session.ID.String()),
		zap.String("user_id", userID.String()),
	)

	return &session, nil
}

func (r *RedisRepo) GetByID(id uuid.UUID) (*Model, error) {
	key := fmt.Sprintf("%s:%s", redisSessionPrefix, id.String())
	sessionJSON, err := r.redisClient.Get(context.Background(), key).Result()

	if err == redis.Nil {
		zap.L().
			Debug("Session not found", zap.String("session_id", id.String()))
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var session Model
	err = json.Unmarshal([]byte(sessionJSON), &session)
	if err != nil {
		zap.L().Error("Failed to unmarshal session", zap.Error(err))
		return nil, err
	}
	zap.L().Debug("Session retrieved", zap.String("session_id", id.String()))

	return &session, nil
}

func (r *RedisRepo) DeleteByID(id uuid.UUID) error {
	key := fmt.Sprintf("%s:%s", redisSessionPrefix, id.String())

	err := r.redisClient.Del(context.Background(), key).Err()
	if err != nil {
		zap.L().Error("Failed to delete session from Redis", zap.Error(err))
		return err
	}

	zap.L().Debug("Session deleted", zap.String("session_id", id.String()))

	return nil
}
