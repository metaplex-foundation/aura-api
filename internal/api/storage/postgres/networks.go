package postgres

import (
	"context"
)

type (
	Network struct {
		ID   int64  `pg:"ntw_id" json:"-"`
		Name string `pg:"ntw_name" json:"name"`
	}
)

const (
	networksTable = "networks"
)

func (s *Storage) GetAvailableNetworks(ctx context.Context) (n map[string]int64, err error) {
	var networks []Network
	query := `SELECT ntw_id, ntw_name FROM networks`
	_, err = s.db.QueryContext(ctx, &networks, query)
	if err != nil {
		return n, err
	}
	n = make(map[string]int64, len(networks))
	for _, network := range networks {
		n[network.Name] = network.ID
	}

	return n, nil
}
