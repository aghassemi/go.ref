package rt

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	isecurity "veyron.io/veyron/veyron/runtimes/google/security"
	vsecurity "veyron.io/veyron/veyron/security"
	"veyron.io/veyron/veyron/security/agent"

	"veyron.io/veyron/veyron2/options"
	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/vlog"
)

func (rt *vrt) Principal() security.Principal {
	return rt.principal
}

func (rt *vrt) NewIdentity(name string) (security.PrivateID, error) {
	return isecurity.NewPrivateID(name, nil)
}

func (rt *vrt) Identity() security.PrivateID {
	return rt.id
}

func (rt *vrt) PublicIDStore() security.PublicIDStore {
	return rt.store
}

func (rt *vrt) initSecurity(credentials string) error {
	if err := rt.initOldSecurity(); err != nil {
		return err
	}
	if err := rt.initPrincipal(credentials); err != nil {
		return fmt.Errorf("principal initialization failed: %v", err)
	}
	return nil
}

func (rt *vrt) initPrincipal(credentials string) error {
	if rt.principal != nil {
		return nil
	}
	var err error
	// TODO(cnicolaou,ashankar,ribrdb): this should be supplied via
	// the exec.GetChildHandle call.
	if len(os.Getenv(agent.FdVarName)) > 0 {
		rt.principal, err = rt.connectToAgent()
		return err
	} else if len(credentials) > 0 {
		// TODO(ataly, ashankar): If multiple runtimes are getting
		// initialized at the same time from the same VEYRON_CREDENTIALS
		// we will need some kind of locking for the credential files.
		if rt.principal, err = vsecurity.LoadPersistentPrincipal(credentials, nil); err != nil {
			if os.IsNotExist(err) {
				if rt.principal, err = vsecurity.CreatePersistentPrincipal(credentials, nil); err != nil {
					return err
				}
				return vsecurity.InitDefaultBlessings(rt.principal, defaultBlessingName())
			}
			return err
		}
		return nil
	}
	if rt.principal, err = vsecurity.NewPrincipal(); err != nil {
		return err
	}
	return vsecurity.InitDefaultBlessings(rt.principal, defaultBlessingName())
}

// TODO(ataly, ashankar): Get rid of this method once we get rid of
// PrivateID and PublicIDStore.
func (rt *vrt) initOldSecurity() error {
	if err := rt.initIdentity(); err != nil {
		return err
	}
	if err := rt.initPublicIDStore(); err != nil {
		return err
	}
	if err := rt.store.Add(rt.id.PublicID(), security.AllPrincipals); err != nil {
		return fmt.Errorf("could not initialize a PublicIDStore for the runtime: %s", err)
	}
	trustIdentityProviders(rt.id)
	return nil
}

func (rt *vrt) initIdentity() error {
	if rt.id != nil {
		return nil
	}
	var err error
	if file := os.Getenv("VEYRON_IDENTITY"); len(file) > 0 {
		if rt.id, err = loadIdentityFromFile(file); err != nil || rt.id == nil {
			return fmt.Errorf("Could not load identity from the VEYRON_IDENTITY environment variable (%q): %v", file, err)
		}
	} else {
		name := defaultBlessingName()
		vlog.VI(2).Infof("No identity provided to the runtime, minting one for %q", name)
		if rt.id, err = rt.NewIdentity(name); err != nil || rt.id == nil {
			return fmt.Errorf("Could not create new identity: %v", err)
		}
	}
	return nil
}

func (rt *vrt) initPublicIDStore() error {
	if rt.store != nil {
		return nil
	}

	var backup *isecurity.PublicIDStoreParams
	// TODO(ataly): Get rid of this environment variable and have a single variable for
	// all security related initialization.
	if dir := os.Getenv("VEYRON_PUBLICID_STORE"); len(dir) > 0 {
		backup = &isecurity.PublicIDStoreParams{dir, rt.id}
	}
	var err error
	if rt.store, err = isecurity.NewPublicIDStore(backup); err != nil {
		return fmt.Errorf("Could not create PublicIDStore for runtime: %v", err)
	}
	return nil
}

func defaultBlessingName() string {
	var name string
	if user, _ := user.Current(); user != nil && len(user.Username) > 0 {
		name = user.Username
	} else {
		name = "anonymous"
	}
	if host, _ := os.Hostname(); len(host) > 0 {
		name = name + "@" + host
	}
	return fmt.Sprintf("%s-%d", name, os.Getpid())
}

func trustIdentityProviders(id security.PrivateID) { isecurity.TrustIdentityProviders(id) }

func loadIdentityFromFile(filePath string) (security.PrivateID, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// TODO(ashankar): Hack. See comments in vsecurity.LoadIdentity.
	hack, err := isecurity.NewPrivateID("hack", nil)
	if err != nil {
		return nil, err
	}
	return vsecurity.LoadIdentity(f, hack)
}

func (rt *vrt) connectToAgent() (security.Principal, error) {
	client, err := rt.NewClient(options.VCSecurityNone)
	if err != nil {
		return nil, err
	}
	fd, err := strconv.Atoi(os.Getenv(agent.FdVarName))
	if err != nil {
		return nil, err
	}
	return agent.NewAgentPrincipal(client, fd, rt.NewContext())
}
