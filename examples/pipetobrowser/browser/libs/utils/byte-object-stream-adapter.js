import { default as Stream } from "nodelibs/stream"
import { default as buffer } from "nodelibs/buffer"

var Transform = Stream.Transform;
var Buffer = buffer.Buffer;

/*
 * Adapts a stream of byte arrays in object mode to a regular stream of Buffer
 * @class
 */
export class ByteObjectStreamAdapter extends Transform {
  constructor() {
    super();
    this._writableState.objectMode = true;
    this._readableState.objectMode = false;
  }

  _transform(bytesArr, encoding, cb) {
    var buf = new Buffer(new Uint8Array(bytesArr));
    this.push(buf);

    cb();
  }
}
