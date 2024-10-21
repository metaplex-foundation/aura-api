package util

import (
	"encoding/json"
	"io"

	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/pkg/log"
)

func Map[T any, I any](ss []T, callback func(T) I) []I {
	if len(ss) == 0 {
		return nil
	}

	ret := make([]I, 0, len(ss))
	for i := range ss {
		ret = append(ret, callback(ss[i]))
	}

	return ret
}

func ConvertMap[K comparable, X any, Y any](m map[K]X, callback func(X) Y) map[K]Y {
	if len(m) == 0 {
		return nil
	}

	ret := make(map[K]Y, len(m))
	for i, v := range m {
		ret[i] = callback(v)
	}

	return ret
}

func NewJSONDecoder(data io.Reader, disallowUnknownFields bool) (decoder *json.Decoder) {
	decoder = json.NewDecoder(data)
	decoder.UseNumber()
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	return
}

func ParseUUIDOrDefault(u string) uuid.UUID {
	res, err := uuid.Parse(u)
	if err != nil {
		log.Logger.General.Errorf("ParseUUIDOrDefault (%s): %s", u, err)
		return uuid.UUID{}
	}

	return res
}

func Chunk[T any](slice []T, chunkSize int) (chunks [][]T) {
	for {
		if len(slice) == 0 {
			break
		}

		if len(slice) < chunkSize {
			chunkSize = len(slice)
		}

		chunks = append(chunks, slice[0:chunkSize])
		slice = slice[chunkSize:]
	}

	return
}
