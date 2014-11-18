package browspr

import (
	"veyron.io/wspr/veyron/services/wsprd/app"
	"veyron.io/wspr/veyron/services/wsprd/lib"
)

// postMessageWriter is a lib.ClientWriter that handles sending messages over postMessage to the extension.
type postMessageWriter struct {
	messageId int64
	p         *pipe
}

func (w *postMessageWriter) Send(messageType lib.ResponseType, data interface{}) error {
	outMsg, err := app.ConstructOutgoingMessage(w.messageId, messageType, data)
	if err != nil {
		return err
	}

	w.p.browspr.postMessage(w.p.instanceId, "browsprMsg", outMsg)
	return nil
}

func (w *postMessageWriter) Error(err error) {
	w.Send(lib.ResponseError, app.FormatAsVerror(err, w.p.browspr.logger))
}
