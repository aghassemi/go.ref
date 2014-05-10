(function(exports) {
/**
 * MountPoint handles manipulating and querying from
 * a mounttable.
 * @param {object} client A veyron client.
 * @param {...string} addressParts Parts of the address to join
 * @constructor
 */
var MountPoint = function(client, addressParts) {
  this.client = client
  this.addr = Array.prototype.slice.call(arguments, 1).join('/');
}

/**
 * appendToPath appends to the mountpoint path
 * @param {...string} toAdd strings to add to the path.
 * @return {MountPoint} a new mount point with the path args appended
 * to the current path.
 */
MountPoint.prototype.appendToPath = function(toAdd) {
  var args = Array.prototype.slice.call(arguments);
  args.unshift(this.addr);
  return new MountPoint(this.client, args.join('/'));
}

/**
 * mount mounts a target to the current mount point.
 * @param {string} target The target to be mounted.
 * @return {promise} a promise that completes when it is mounted
 */
MountPoint.prototype.mount = function(target) {
  return this.client.bind(this.addr).then(function(mtService) {
    return mtService.mount(target, 0);
  });
}

/**
 * glob makes a glob request to a server relative to the current mountpoint.
 * @param {string} expr The glob expression e.g. A/B/*.
 * @return {promise} a promise to a list of results
 */
MountPoint.prototype.glob = function(expr) {
  var results = [];
  return this.client.bind(this.addr).then(function(mtService) {
    var promise = mtService.glob(expr);
    var stream = promise.stream;

    stream.on('readable', function() {
      var val = stream.read();
      if (val) {
        results.push(val);
      }
    });

    return promise.then(function() {
      return results;
    });
  });
};

/**
 * isMounttable determines if a specific address refers to a
 * mounttable.
 * @param {object} client the veyron client to use.
 * @param {string} globResult result of glob to check.
 * @return {promise} promise to a boolean indicating if it is
 * a mounttable.
 */
 // TODO(bprosnitz) Remove dependency on _proxyConnection.
 // TODO(bprosnitz) Consider adding interface name to signature and using that.
var isMounttable = function(client, globResult) {
  if (globResult.servers.length === 0) {
    // This is on the same mounttable as the globResult.
    return new Promise(function(resolve) {
      resolve(true);
    });
  }

  var pconn = client._proxyConnection;
  var promises = [];
  for (var i in globResult.servers) {
    promises.push((function(server) {
      return pconn.getServiceSignature(server)
    })(globResult.servers[i].server).then(function(sig) {
      if (sig['glob'] !== undefined) {
        if (sig['glob'].inArgs.length == 1) {
          return true;
        }
      }
      return false;
    }));
  }

  return resolveRace(promises);
}

exports.MountPoint = MountPoint;
exports.isMounttable = isMounttable;
})(window);