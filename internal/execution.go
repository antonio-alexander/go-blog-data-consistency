package internal

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

//RowsAffected can be used to return a pre-determined error via errorString in the event
// no rows are affected; this function assumes that in the event no error is returned and
// rows were supposed to be affected, an error will be returned
func RowsAffected(result sql.Result, errorString string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return nil
	}
	if n <= 0 {
		return errors.New(errorString)
	}
	return nil
}

//GenerateID can be used to generate a v4 uuid and return
// it as a string, if it's unable to, it'll panic
func GenerateID() string {
	return uuid.Must(uuid.NewRandom()).String()
}
