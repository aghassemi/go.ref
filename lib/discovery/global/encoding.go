// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"strconv"
	"strings"

	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/vom"
)

// encodeAdToSuffix encodes the ad.Id and the ad.Attributes into the suffix at
// which we mount the advertisement.
// The format of the generated suffix is id/timestamp/attributes.
//
// TODO(suharshs): Currently only the id and the attributes are encoded; we may
// want to encode the rest of the advertisement someday?
func encodeAdToSuffix(ad *discovery.Advertisement, timestampNs int64) (string, error) {
	b, err := vom.Encode(ad.Attributes)
	if err != nil {
		return "", err
	}
	// Escape suffixDelim to use it as our delimeter between the id and the attrs.
	id := ad.Id.String()
	timestamp := strconv.FormatInt(timestampNs, 10)
	attr := naming.EncodeAsNameElement(string(b))
	return naming.Join(id, timestamp, attr), nil
}

// decodeAdFromSuffix decodes in into an advertisement.
// The format of the input suffix is id/timestamp/attributes.
func decodeAdFromSuffix(in string) (*discovery.Advertisement, int64, error) {
	parts := strings.SplitN(in, "/", 3)
	if len(parts) != 3 {
		return nil, 0, NewErrAdInvalidEncoding(nil, in)
	}
	var err error
	ad := &discovery.Advertisement{}
	if ad.Id, err = discovery.ParseAdId(parts[0]); err != nil {
		return nil, 0, err
	}
	timestampNs, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, 0, err
	}
	attrs, ok := naming.DecodeFromNameElement(parts[2])
	if !ok {
		return nil, 0, NewErrAdInvalidEncoding(nil, in)
	}
	if err = vom.Decode([]byte(attrs), &ad.Attributes); err != nil {
		return nil, 0, err
	}
	return ad, timestampNs, nil
}
