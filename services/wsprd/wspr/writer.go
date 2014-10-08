package wspr

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"

	"veyron.io/wspr/veyron/services/wsprd/lib"

	"veyron.io/veyron/veyron2/verror2"
	"veyron.io/veyron/veyron2/vlog"
	"veyron.io/veyron/veyron2/vom"

	"github.com/gorilla/websocket"
)

// Wraps a response to the proxy client and adds a message type.
type response struct {
	Type    lib.ResponseType
	Message interface{}
}

// Implements clientWriter interface for sending messages over websockets.
type websocketWriter struct {
	p      *pipe
	logger vlog.Logger
	id     int64
}

func (w *websocketWriter) Send(messageType lib.ResponseType, data interface{}) error {
	var buf bytes.Buffer
	if err := vom.ObjToJSON(&buf, vom.ValueOf(response{Type: messageType, Message: data})); err != nil {
		w.logger.Error("Failed to marshal with", err)
		return err
	}

	var buf2 bytes.Buffer

	if err := vom.ObjToJSON(&buf2, vom.ValueOf(websocketMessage{Id: w.id, Data: buf.String()})); err != nil {
		w.logger.Error("Failed to write the message", err)
		return err
	}

	w.p.writeQueue <- wsMessage{messageType: websocket.TextMessage, buf: buf2.Bytes()}

	return nil
}

func (w *websocketWriter) Error(err error) {
	verr := verror2.Convert(verror2.Unknown, nil, err)

	// Also log the error but write internal errors at a more severe log level
	var logLevel vlog.Level = 2
	logErr := fmt.Sprintf("%v", verr)

	// Prefix the message with the code locations associated with verr,
	// except the last, which is the Convert() above.  This does nothing if
	// err was not a verror2 error.
	verrStack := verror2.Stack(verr)
	for i := 0; i < len(verrStack)-1; i++ {
		pc := verrStack[i]
		fnc := runtime.FuncForPC(pc)
		file, line := fnc.FileLine(pc)
		logErr = fmt.Sprintf("%s:%d: %s", file, line)
	}

	// We want to look at the stack three frames up to find where the error actually
	// occurred.  (caller -> websocketErrorResponse/sendError -> generateErrorMessage).
	if _, file, line, ok := runtime.Caller(3); ok {
		logErr = fmt.Sprintf("%s:%d: %s", filepath.Base(file), line, logErr)
	}
	if verror2.Is(verr, verror2.Internal.ID) {
		logLevel = 2
	}
	w.logger.VI(logLevel).Info(logErr)

	w.Send(lib.ResponseError, verr)
}
