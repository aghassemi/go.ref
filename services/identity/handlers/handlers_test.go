package handlers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"veyron.io/veyron/veyron/services/identity/util"

	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/security"
)

func TestPublicKey(t *testing.T) {
	r, err := rt.New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Cleanup()
	ts := httptest.NewServer(PublicKey{r.Identity().PublicID()})
	defer ts.Close()
	response, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := security.UnmarshalPublicKey(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if want := r.Identity().PublicKey(); !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
}

func TestRandom(t *testing.T) {
	r, err := rt.New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Cleanup()
	ts := httptest.NewServer(Random{r})
	defer ts.Close()

	got, err := parseResponse(http.Get(ts.URL))
	if err != nil {
		t.Fatal(err)
	}
	if id, ok := got.(security.PrivateID); !ok {
		t.Fatalf("Got %T want security.PrivateID", got, id)
	}
}

func TestBless(t *testing.T) {
	r, err := rt.New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Cleanup()

	ts := httptest.NewServer(http.HandlerFunc(Bless))
	defer ts.Close()

	// GET requests should succeed (render the form)
	if resp, err := http.Get(ts.URL); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("Got (%+v, %v) want (200, nil)", resp, err)
	}

	blessor, err := r.NewIdentity("god")
	if err != nil {
		t.Fatal(err)
	}
	blessee, err := r.NewIdentity("person")
	if err != nil {
		t.Fatal(err)
	}

	bless := func(blesser security.PrivateID, blessee security.PublicID, name string) security.PublicID {
		blessedID, err := blesser.Bless(blessee, name, 24*time.Hour, nil)
		if err != nil {
			t.Fatalf("%q.Bless(%q, %q, ...) failed: %v", blesser, blessee, name, err)
		}
		return blessedID
	}

	tests := []struct {
		Blessor, Blessee  interface{}
		BlessingName      string
		ExpectedBlessedID security.PublicID
	}{
		{ // No field specified, bad request
			Blessor: nil,
			Blessee: nil,
		},
		{ // No blessee specified, bad request
			Blessor: blessor,
			Blessee: nil,
		},
		{ // No blessor specified, bad request
			Blessor: nil,
			Blessee: blessee,
		},
		{ // No name specified, bad request
			Blessor: blessor,
			Blessee: blessee,
		},
		{ // Blessor is a security.PublicID, bad request
			Blessor:      blessor.PublicID(),
			Blessee:      blessee,
			BlessingName: "batman",
		},
		{ // Everything specified, blessee is a security.PrivateID. Should succeed
			Blessor:           blessor,
			Blessee:           blessee,
			BlessingName:      "batman",
			ExpectedBlessedID: bless(blessor, blessee.PublicID(), "batman"),
		},
		{ // Everything specified, blessee is a security.PublicID. Should succeed
			Blessor:           blessor,
			Blessee:           blessee.PublicID(),
			BlessingName:      "batman",
			ExpectedBlessedID: bless(blessor, blessee.PublicID(), "batman"),
		},
	}
	for _, test := range tests {
		debug := fmt.Sprintf("%q.Bless(%q, %q, ...)", test.Blessor, test.Blessee, test.BlessingName)
		v := url.Values{}
		if test.Blessor != nil {
			v.Set("blessor", b64vomencode(test.Blessor))
		} else {
			v.Set("blessor", "")
		}
		if test.Blessee != nil {
			v.Set("blessee", b64vomencode(test.Blessee))
		} else {
			v.Set("blessee", "")
		}
		v.Set("name", test.BlessingName)
		res, err := http.PostForm(ts.URL, v)
		if test.ExpectedBlessedID == nil {
			if res.StatusCode != http.StatusBadRequest {
				t.Errorf("%v: Got (%v=%v) want 400", debug, res.StatusCode, res.Status)
			}
			continue
		}
		id, err := parseResponse(res, nil)
		if err != nil {
			t.Errorf("%v error: %v", debug, err)
			continue
		}
		pub, ok := id.(security.PublicID)
		if !ok {
			t.Errorf("%v returned %T, want security.PublicID", debug, id)
			continue
		}
		if got, want := fmt.Sprintf("%s", pub), fmt.Sprintf("%s", test.ExpectedBlessedID); got != want {
			t.Errorf("%v returned an identity %q want %q", debug, got, want)
			continue
		}
	}
}

func parseResponse(r *http.Response, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}
	b64, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var parsed interface{}
	if err := util.Base64VomDecode(string(b64), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func b64vomencode(obj interface{}) string {
	str, err := util.Base64VomEncode(obj)
	if err != nil {
		panic(err)
	}
	return str
}
