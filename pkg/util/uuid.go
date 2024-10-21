package util

import "github.com/google/uuid"

func GenerateUUIDWithRetry() (u uuid.UUID, err error) {
	for i := 0; i < 5; i++ {
		u, err = uuid.NewRandom()
		if err == nil {
			return u, nil
		}
	}

	return
}
