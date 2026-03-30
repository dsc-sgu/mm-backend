package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("lock not acquired")

type Manager interface {
	Acquire(
		ctx context.Context,
		courseID, userID uuid.UUID,
		ttl time.Duration,
	) (bool, error)
	Release(ctx context.Context, courseID, userID uuid.UUID) error
	Heartbeat(
		ctx context.Context,
		courseID, userID uuid.UUID,
		ttl time.Duration,
	) error
	GetLocker(ctx context.Context, courseID uuid.UUID) (uuid.UUID, error)
}

type RedisManager struct {
	redisClient *redis.Client
}

func NewRedisManager(redisClient *redis.Client) Manager {
	return &RedisManager{redisClient}
}

func (r *RedisManager) key(courseID uuid.UUID) string {
	return fmt.Sprintf("lock:course:%s", courseID.String())
}

func (r *RedisManager) Acquire(
	ctx context.Context,
	courseID, userID uuid.UUID,
	ttl time.Duration,
) (bool, error) {
	script := `
	return redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2], "NX")
	`
	res, err := r.redisClient.Eval(ctx, script, []string{r.key(courseID)}, userID.String(), int(ttl.Seconds())).
		Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// This can happen if the script returns nil, which for SET NX means the key was not set.
			return false, nil
		}
		return false, err
	}

	// If the key was set, Redis returns "OK". If not, it returns nil.
	return res != nil, nil
}

func (r *RedisManager) Release(
	ctx context.Context,
	courseID, userID uuid.UUID,
) error {
	// We use a script to ensure that we only delete the key if it is owned by the user.
	// This prevents a user from releasing a lock that has been acquired by another user
	// after the first user's lock has expired.
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
	`
	res, err := r.redisClient.Eval(ctx, script, []string{r.key(courseID)}, userID.String()).
		Result()
	if err != nil {
		return err
	}

	if res.(int64) == 0 {
		return ErrLockNotAcquired
	}

	return nil
}

func (r *RedisManager) Heartbeat(
	ctx context.Context,
	courseID, userID uuid.UUID,
	ttl time.Duration,
) error {
	// We use a script to ensure that we only extend the lock if it is owned by the user.
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("expire", KEYS[1], ARGV[2])
	else
		return 0
	end
	`
	res, err := r.redisClient.Eval(ctx, script, []string{r.key(courseID)}, userID.String(), int(ttl.Seconds())).
		Result()
	if err != nil {
		return err
	}

	if res.(int64) == 0 {
		return ErrLockNotAcquired
	}

	return nil
}

func (r *RedisManager) GetLocker(
	ctx context.Context,
	courseID uuid.UUID,
) (uuid.UUID, error) {
	val, err := r.redisClient.Get(ctx, r.key(courseID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, nil // No lock
		}
		return uuid.Nil, err
	}

	lockerID, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse locker user ID: %w", err)
	}

	return lockerID, nil
}
