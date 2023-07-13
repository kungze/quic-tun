package tunnel

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

type tunnelDataStore struct {
	sync.Map
}

func (t *tunnelDataStore) LoadAll() []tunnel {
	var tunnels []tunnel
	t.Range(func(key, value any) bool {
		tunnels = append(tunnels, value.(tunnel))
		return true
	})
	return tunnels
}

func (t *tunnelDataStore) LoadOne(uuid uuid.UUID) (tunnel, error) {
	var tun tunnel
	value, ok := t.Load(uuid)
	if ok {
		tun, ok = value.(tunnel)
		if ok {
			return tun, nil
		}
	}
	return tun, errors.New("not found tunnel for uuid")
}

// Used to store all active tunnels information
var DataStore = tunnelDataStore{}
