// Package core is what gomobile binds for Android: Open, one Call that takes a
// method name and JSON, the events back through a Listener, the backup bytes,
// and Close. gobind carries only basic types and []byte, so this package is a
// shell and every real type lives in mobile/engine.
//
// Kotlin sees it as com.nxck.surmise.engine.core (`-javapkg
// com.nxck.surmise.engine`, see scripts/build-android-core.sh).
package core

import (
	_ "time/tzdata" // Go on Android finds no zone database; the zone comes from Kotlin

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/online"
	"github.com/nxck2005/surmise/mobile/engine"
)

// Listener receives events from the engine, on Go's own threads.
type Listener interface {
	OnEvent(kind string, payload []byte)
}

// Engine is one open install.
type Engine struct {
	e *engine.Engine
}

// Open opens the install in dataDir. zone is the device's IANA zone name,
// version the app's version name and abi the device's primary ABI; l may be
// nil.
func Open(dataDir, zone, version, abi string, l Listener) (*Engine, error) {
	c := engine.Config{
		DataDir: dataDir, Zone: zone, Version: version, ABI: abi,
		Client: online.ClientName(brand.Name, version, "android", abi),
	}
	if l != nil {
		c.Emit = l.OnEvent
	}
	e, err := engine.Open(c)
	if err != nil {
		return nil, err
	}
	return &Engine{e: e}, nil
}

// Call runs one method: JSON in, a JSON envelope out ({"ok": …} or
// {"err": {"code", "detail"}}).
func (e *Engine) Call(method string, request []byte) []byte {
	return e.e.Call(method, request)
}

// SetZone changes the device zone, on ACTION_TIMEZONE_CHANGED.
func (e *Engine) SetZone(zone string) error { return e.e.SetZone(zone) }

// ExportBackup builds a backup archive of the whole install.
func (e *Engine) ExportBackup() ([]byte, error) { return e.e.Export() }

// ImportBackup merges an archive into the install and answers with an
// envelope holding the report.
func (e *Engine) ImportBackup(b []byte) []byte { return e.e.Import(b) }

// Close banks and saves the open board.
func (e *Engine) Close() error { return e.e.Close() }
