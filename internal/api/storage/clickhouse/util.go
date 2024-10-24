package clickhouse

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrEmptyUserUUID = errors.New("empty userUID")

type Date struct {
	time.Time
}

func (d *Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Time.Format("2006-01-02"))
}
