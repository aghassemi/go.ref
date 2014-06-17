/*
 * Implements and publishes a Veyron service which accepts streaming RPC
 * requests and delegates the stream back to the provided pipeRequestHandler.
 * It also exposes the state of the service.
 * @fileoverview
 */

import { Logger } from 'libs/logs/logger'
import { config } from 'config'
import { ByteObjectStreamAdapter } from 'libs/utils/byte-object-stream-adapter'

var log = new Logger('services/p2b');
var v = new Veyron(config.veyron);
var server = v.newServer();

// State of p2b service
export var state = {
  init() {
    this.published = false;
    this.publishing = false;
    this.stopping = false;
    this.fullServiceName = null;
    this.date = null;
    this.numPipes = 0;
    this.numBytes = 0;
  },
  reset() {
    state.init();
  }
};
state.init();

/*
 * Publishes the p2b service under google/p2b/{name}
 * e.g. If name is "JohnTablet", p2b service will be accessible under name:
 * 'google/p2b/JohnTablet'
 *
 * pipe() method can be invoked on any 'google/p2b/{name}/suffix' name where
 * suffix identifies the viewer that can format and display the stream data
 * e.g. 'google/p2b/JohnTablet/DataTable'.pipe() will display the incoming
 * data in a data table. See /app/viewer/ for a list of available viewers.
 * @param {string} name Name to publish the service under
 * @param {function} pipeRequestHandler A function that will be called when
 * a request to handle a pipe stream comes in.
 * @return {Promise} Promise that will resolve or reject when publish completes
 */
export function publish(name, pipeRequestHandler) {
  log.debug('publishing under name:', name);

  /*
   * Veyron pipe to browser service implementation.
   * Implements the p2b IDL.
   */
  var p2b = {
    pipe($suffix, $stream) {
      return new Promise(function(resolve, reject) {
        log.debug('received pipe request for:', $suffix);

        var bufferStream = new ByteObjectStreamAdapter();
        $stream.pipe(bufferStream);

        bufferStream.on('end', () => {
          log.debug('end of stream');
          resolve('done');
        });

        bufferStream.on('error', (e) => {
          log.debug('stream error', e);
          reject(e);
        });

        state.numPipes++;
        //TODO(aghassemi) pipe to a byte-size-sniffer to update state.numBytes

        pipeRequestHandler($suffix, bufferStream);
      });
    }
  };

  state.publishing = true;

  return server.register(name, p2b).then(() => {
    return server.publish(config.publishNamePrefix).then((endpoint) => {
      log.debug('published with endpoint:', endpoint);

      state.published = true;
      state.publishing = false;
      state.fullServiceName = config.publishNamePrefix + '/' + name;
      state.date = new Date();

      return endpoint;
    });
  }).catch((err) => { state.reset(); throw err; });
}

/*
 * Stops the service and unpublishes it, effectively destroying the service.
 * @return {Promise} Promise that will resolve or reject when stopping completes
 */
export function stopPublishing() {
  log.debug('stopping service');
  state.stopping = true;
  return server.stop().then(function() {
    log.debug('service stopped');
    state.reset();
    return;
  });
}