package dtmx

import (
	"sort"
	"sync"
)

type DriverOpener func(Config) (Manager, error)

var (
	driversMu sync.RWMutex
	drivers   = map[string]DriverOpener{}
)

func RegisterDriver(name string, opener DriverOpener) {
	if name == "" || opener == nil {
		panic("dtmx: driver name and opener are required")
	}
	driversMu.Lock()
	defer driversMu.Unlock()
	if _, ok := drivers[name]; ok {
		panic("dtmx: driver " + name + " already registered")
	}
	drivers[name] = opener
}

func IsDriverRegistered(name string) bool {
	driversMu.RLock()
	defer driversMu.RUnlock()
	_, ok := drivers[name]
	return ok
}

func RegisteredDrivers() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	out := make([]string, 0, len(drivers))
	for name := range drivers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
