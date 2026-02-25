package bang

import (
	"context"
	"errors"
	"strings"
)

// WrapDBError wraps a database error into an appropriate bang class.
// Returns nil if err is nil.
//
//	bang.WrapDBError(err).Code("users.repo.get").Debug("id", id)
func WrapDBError(err error) *Error {
	if err == nil {
		return nil
	}
	return newError(classifyDBError(err), 1).Wrap(err)
}

func classifyDBError(err error) Class {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ClassTimeout
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no rows"):
		return ClassNotFound
	case strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique") || strings.Contains(msg, "23505"):
		return ClassDuplicateKey
	case strings.Contains(msg, "foreign key") || strings.Contains(msg, "23503"):
		return ClassForeignKeyViolation
	default:
		return ClassDatabase
	}
}

// WrapRedisError wraps a Redis client error into an appropriate bang class.
// Returns nil if err is nil.
//
//	bang.WrapRedisError(err).Code("cache.session.get").Debug("key", key)
func WrapRedisError(err error) *Error {
	if err == nil {
		return nil
	}
	return newError(classifyRedisError(err), 1).Wrap(err)
}

func classifyRedisError(err error) Class {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ClassTimeout
	}
	msg := strings.ToLower(err.Error())
	switch {
	case msg == "redis: nil" || strings.Contains(msg, "nil"):
		return ClassNotFound
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "connect:") || strings.Contains(msg, "i/o timeout"):
		return ClassNetwork
	case strings.Contains(msg, "timeout"):
		return ClassTimeout
	default:
		return ClassDatabase
	}
}

// TODO: Правильная реализация для поддержки WrapValidationError(nil) == nil
func WrapValidationError(err error) *Error {
	return bindErr(err)
}
