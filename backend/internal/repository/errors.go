package repository

import (
	"errors"

	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrNotFound = errors.New("repository resource not found")
	ErrConflict = errors.New("repository resource conflict")
)

func mongoError(err error) error {
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return ErrNotFound
	case mongo.IsDuplicateKeyError(err):
		return ErrConflict
	default:
		return err
	}
}
