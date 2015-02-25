package vc

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"v.io/core/veyron/runtimes/google/ipc/stream/crypto"
	"v.io/core/veyron/runtimes/google/lib/iobuf"

	"v.io/v23/context"
	"v.io/v23/ipc/version"
	"v.io/v23/security"
	"v.io/v23/vom"
)

var (
	authServerContextTag = []byte("VCauthS\x00")
	authClientContextTag = []byte("VCauthC\x00")
)

var (
	errSameChannelPublicKey      = errors.New("same public keys for both ends of the channel")
	errChannelIDMismatch         = errors.New("channel id does not match expectation")
	errChecksumMismatch          = errors.New("checksum mismatch")
	errInvalidSignatureInMessage = errors.New("signature does not verify in authentication handshake message")
	errNoCertificatesReceived    = errors.New("no certificates received")
	errSingleCertificateRequired = errors.New("exactly one X.509 certificate chain with exactly one certificate is required")
)

// AuthenticateAsServer executes the authentication protocol at the server and
// returns the blessings used to authenticate the client.
func AuthenticateAsServer(conn io.ReadWriteCloser, principal security.Principal, server security.Blessings, dc DischargeClient, crypter crypto.Crypter, v version.IPCVersion) (client security.Blessings, clientDischarges map[string]security.Discharge, err error) {
	defer conn.Close()
	if server == nil {
		return nil, nil, errors.New("no blessings to present as a server")
	}
	var discharges []security.Discharge
	if tpcavs := server.ThirdPartyCaveats(); len(tpcavs) > 0 && dc != nil {
		discharges = dc.PrepareDischarges(nil, tpcavs, security.DischargeImpetus{})
	}
	if err = writeBlessings(conn, authServerContextTag, crypter, principal, server, discharges, v); err != nil {
		return
	}
	if client, clientDischarges, err = readBlessings(conn, authClientContextTag, crypter, v); err != nil {
		return
	}
	return
}

// AuthenticateAsClient executes the authentication protocol at the client and
// returns the blessings used to authenticate both ends.
//
// The client will only share its identity if its blessing store has one marked
// for the server (who shares its blessings first).
//
// TODO(ashankar): Seems like there is no way the blessing store
// can say that it does NOT want to share the default blessing with the server?
func AuthenticateAsClient(ctx *context.T, conn io.ReadWriteCloser, principal security.Principal, dc DischargeClient, crypter crypto.Crypter, v version.IPCVersion) (server, client security.Blessings, serverDischarges map[string]security.Discharge, err error) {
	defer conn.Close()
	if server, serverDischarges, err = readBlessings(conn, authServerContextTag, crypter, v); err != nil {
		return
	}
	serverB, invalidB := server.ForContext(security.NewContext(&security.ContextParams{
		LocalPrincipal:   principal,
		RemoteBlessings:  server,
		RemoteDischarges: serverDischarges,
		// TODO(ashankar): Get the local and remote endpoint here?
		// There is also a bootstrapping problem here. For example, let's say
		// (1) server has the blessing "provider/server" with a PeerIdentity caveat of "provider/client"
		// (2) Client has a blessing "provider/client" tagged for "provider/server" in its BlessingStore
		// How do we get that working?
		// One option is to have a UnionOfBlessings of all blessings of the client in the BlessingStore
		// made available to serverAuthContext.LocalBlessings for this call.
		Context: ctx,
	}))
	client = principal.BlessingStore().ForPeer(serverB...)
	if client == nil {
		return nil, nil, nil, NewErrNoBlessingsForPeer(ctx, serverB, invalidB)
	}
	var discharges []security.Discharge
	if dc != nil {
		discharges = dc.PrepareDischarges(ctx, client.ThirdPartyCaveats(), security.DischargeImpetus{})
	}
	if err = writeBlessings(conn, authClientContextTag, crypter, principal, client, discharges, v); err != nil {
		return
	}
	return
}

func writeBlessings(w io.Writer, tag []byte, crypter crypto.Crypter, p security.Principal, b security.Blessings, discharges []security.Discharge, v version.IPCVersion) error {
	signature, err := p.Sign(append(tag, crypter.ChannelBinding()...))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	enc, err := vom.NewEncoder(&buf)
	if err != nil {
		return err
	}
	if err := enc.Encode(signature); err != nil {
		return err
	}
	if err := enc.Encode(security.MarshalBlessings(b)); err != nil {
		return err
	}
	if v >= version.IPCVersion7 {
		wired := make([]security.WireDischarge, len(discharges))
		for i, d := range discharges {
			wired[i] = security.MarshalDischarge(d)
		}
		if err := enc.Encode(wired); err != nil {
			return err
		}
	} else if v >= version.IPCVersion5 {
		if err := enc.Encode(discharges); err != nil {
			return err
		}
	}
	msg, err := crypter.Encrypt(iobuf.NewSlice(buf.Bytes()))
	if err != nil {
		return err
	}
	defer msg.Release()
	enc, err = vom.NewEncoder(w)
	if err != nil {
		return err
	}
	return enc.Encode(msg.Contents)
}

func readBlessings(r io.Reader, tag []byte, crypter crypto.Crypter, v version.IPCVersion) (security.Blessings, map[string]security.Discharge, error) {
	var msg []byte
	dec, err := vom.NewDecoder(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new decoder: %v", err)
	}
	if err := dec.Decode(&msg); err != nil {
		return nil, nil, fmt.Errorf("failed to read handshake message: %v", err)
	}
	buf, err := crypter.Decrypt(iobuf.NewSlice(msg))
	if err != nil {
		return nil, nil, err
	}
	defer buf.Release()
	dec, err = vom.NewDecoder(bytes.NewReader(buf.Contents))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new decoder: %v", err)
	}

	var (
		wireb security.WireBlessings
		sig   security.Signature
	)
	if err = dec.Decode(&sig); err != nil {
		return nil, nil, err
	}
	if err = dec.Decode(&wireb); err != nil {
		return nil, nil, err
	}
	var discharges map[string]security.Discharge
	if v >= version.IPCVersion7 {
		var wired []security.WireDischarge
		if err = dec.Decode(&wired); err != nil {
			return nil, nil, err
		}
		if len(wired) > 0 {
			discharges = make(map[string]security.Discharge)
			for _, w := range wired {
				d := security.NewDischarge(w)
				discharges[d.ID()] = d
			}
		}
	} else if v >= version.IPCVersion5 {
		var list []security.Discharge
		if err = dec.Decode(&list); err != nil {
			return nil, nil, err
		}
		if len(list) > 0 {
			discharges = make(map[string]security.Discharge)
			for _, d := range list {
				discharges[d.ID()] = d
			}
		}
	}
	blessings, err := security.NewBlessings(wireb)
	if err != nil {
		return nil, nil, err
	}
	if blessings == nil {
		return nil, nil, errNoCertificatesReceived
	}
	if !sig.Verify(blessings.PublicKey(), append(tag, crypter.ChannelBinding()...)) {
		return nil, nil, errInvalidSignatureInMessage
	}
	return blessings, discharges, nil
}
