package tunnel

import (
	"sync"
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

// Used to store all active tunnels information
var DataStore = tunnelDataStore{}
