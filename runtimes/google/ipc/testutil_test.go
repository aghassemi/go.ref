package ipc

import (
	"reflect"
	"testing"

	"v.io/core/veyron2/security"
	"v.io/core/veyron2/verror"
)

func makeResultPtrs(ins []interface{}) []interface{} {
	outs := make([]interface{}, len(ins))
	for ix, in := range ins {
		typ := reflect.TypeOf(in)
		if typ == nil {
			// Nil indicates interface{}.
			var empty interface{}
			typ = reflect.ValueOf(&empty).Elem().Type()
		}
		outs[ix] = reflect.New(typ).Interface()
	}
	return outs
}

func checkResultPtrs(t *testing.T, name string, gotptrs, want []interface{}) {
	for ix, res := range gotptrs {
		got := reflect.ValueOf(res).Elem().Interface()
		want := want[ix]
		switch g := got.(type) {
		case verror.Standard:
			w, ok := want.(verror.Standard)
			// don't use reflect deep equal on verror's since they contain
			// a list of stack PCs which will be different.
			if !ok {
				t.Errorf("%s result %d got type %T, want %T", name, ix, g, w)
			}
			if !verror.Is(g, w.IDAction.ID) {
				t.Errorf("%s result %d got %v, want %v", name, ix, g, w)
			}
		default:
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s result %d got %v, want %v", name, ix, got, want)
			}
		}

	}
}

func mkCaveat(cav security.Caveat, err error) security.Caveat {
	if err != nil {
		panic(err)
	}
	return cav
}

func bless(blesser, blessed security.Principal, extension string, caveats ...security.Caveat) security.Blessings {
	if len(caveats) == 0 {
		caveats = append(caveats, security.UnconstrainedUse())
	}
	b, err := blesser.Bless(blessed.PublicKey(), blesser.BlessingStore().Default(), extension, caveats[0], caveats[1:]...)
	if err != nil {
		panic(err)
	}
	return b
}
