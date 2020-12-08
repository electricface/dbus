package mydbus

import (
	"github.com/electricface/dbus"
	"sync"
)

var sessionBusLck sync.Mutex
var sessionBus *dbus.Conn

var sessionHandler = newHandler()
var systemHandler = newHandler()

func GetSessionHandler() *Handler {
	return sessionHandler
}

func GetSystemHandler() *Handler {
	return systemHandler
}

// SessionBus returns a shared connection to the session bus, connecting to it
// if not already done.
func SessionBus() (conn *dbus.Conn, err error) {
	sessionBusLck.Lock()
	defer sessionBusLck.Unlock()
	if sessionBus != nil {
		return sessionBus, nil
	}
	defer func() {
		if conn != nil {
			sessionBus = conn
		}
	}()
	conn, err = dbus.ConnectSessionBus(dbus.WithHandler(sessionHandler))
	return
}

var systemBusLck sync.Mutex
var systemBus *dbus.Conn

// SystemBus returns a shared connection to the system bus, connecting to it if
// not already done.
func SystemBus() (conn *dbus.Conn, err error) {
	systemBusLck.Lock()
	defer systemBusLck.Unlock()
	if systemBus != nil {
		return systemBus, nil
	}
	defer func() {
		if conn != nil {
			systemBus = conn
		}
	}()
	conn, err = dbus.ConnectSystemBus(dbus.WithHandler(systemHandler))
	return
}
