package online

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// The heartbeat of one room connection. Variables rather than constants so a
// test can run it in milliseconds.
//
// Your own connection reads everything on one socket; a `ping` every
// heartbeat is what keeps the middle quiet, and a server that has stopped
// answering is a dead connection however the TCP stack feels about it.
var (
	pingEvery = 30 * time.Second
	deadAfter = 70 * time.Second
)

// wsConn is one open connection to a room, carrying the heartbeat the
// protocol asks the client to keep.
//
// The intervals are captured here rather than re-read per tick: a goroutine
// reading the variables at its own rhythm races with the test that resets
// them, and one connection's answer stays the same for its whole life.
type wsConn struct {
	c        *websocket.Conn
	ping     time.Duration
	dead     time.Duration
	lastPong atomic.Int64 // UnixNano of the last pong (or of the dial)
	stop     chan struct{}
	once     sync.Once
	spent    atomic.Bool
	err      error // the first Close's error, returned once and then forgotten
}

func newWSConn(conn *websocket.Conn) *wsConn {
	w := &wsConn{
		c:    conn,
		ping: pingEvery,
		dead: deadAfter,
		stop: make(chan struct{}),
	}
	w.lastPong.Store(time.Now().UnixNano())
	go w.heartbeat()
	return w
}

// heartbeat sends the text frame `ping` every interval; a socket that has not
// answered for the dead window is closed out from under its reader, which is
// how Receive learns the connection is over.
func (w *wsConn) heartbeat() {
	ticker := time.NewTicker(w.ping)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			if time.Since(time.Unix(0, w.lastPong.Load())) > w.dead {
				w.c.CloseNow()
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := w.c.Write(ctx, websocket.MessageText, []byte("ping"))
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// Send writes one message.
func (w *wsConn) Send(ctx context.Context, m ClientMsg) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return w.c.Write(ctx, websocket.MessageText, b)
}

// Receive blocks for the next message from the room. The heartbeat's "pong"
// and frames that are not JSON objects are skipped, never returned. An error
// means the connection is over.
func (w *wsConn) Receive(ctx context.Context) (ServerMsg, error) {
	for {
		typ, data, err := w.c.Read(ctx)
		if err != nil {
			return ServerMsg{}, err
		}
		if typ != websocket.MessageText {
			continue
		}
		if string(data) == "pong" {
			w.lastPong.Store(time.Now().UnixNano())
			continue
		}
		var msg ServerMsg
		if json.Unmarshal(data, &msg) != nil || msg.T == "" {
			continue
		}
		return msg, nil
	}
}

// Close ends the connection: the heartbeat first, so it does not write into a
// closed socket on its way out, then a normal close of the socket itself. The
// first call returns the close's error; every later one is nil, and the close
// happens exactly once.
func (w *wsConn) Close() error {
	w.once.Do(func() {
		close(w.stop)
		w.err = w.c.Close(websocket.StatusNormalClosure, "")
	})
	// After the once body has finished, its writes are visible to whatever
	// calls Close next. The first caller takes the error and spends the flag;
	// the rest see it spent and answer nil.
	if w.spent.Swap(true) {
		return nil
	}
	return w.err
}
