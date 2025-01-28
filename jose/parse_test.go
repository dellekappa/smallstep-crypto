package jose

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/smallstep/assert"
	"github.com/stretchr/testify/require"
	"go.step.sm/crypto/pemutil"
	"go.step.sm/crypto/x25519"
)

const (
	ecdsaPublicKey keyType = iota
	ecdsaPrivateKey
	ed25519PublicKey
	ed25519PrivateKey
	rsaPublicKey
	rsaPrivateKey
	octKey
)

type wrapSigner struct {
	crypto.Signer
}

func (w wrapSigner) Public() crypto.PublicKey {
	if w.Signer == nil {
		return nil
	}
	return w.Signer.Public()
}

func (w wrapSigner) Sign(r io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if w.Signer == nil {
		return nil, errors.New("not implemented")
	}
	return w.Signer.Sign(r, digest, opts)
}

type testdata struct {
	typ       keyType
	encrypted bool
}

var files = map[string]testdata{
	"testdata/oct.json":           {octKey, false},
	"testdata/oct.enc.json":       {octKey, true},
	"testdata/okp.pub.json":       {ed25519PublicKey, false},
	"testdata/okp.priv.json":      {ed25519PrivateKey, false},
	"testdata/okp.enc.priv.json":  {ed25519PrivateKey, true},
	"testdata/p256.pub.json":      {ecdsaPublicKey, false},
	"testdata/p256.priv.json":     {ecdsaPrivateKey, false},
	"testdata/p256.enc.priv.json": {ecdsaPrivateKey, true},
	"testdata/rsa.pub.json":       {rsaPublicKey, false},
	"testdata/rsa.priv.json":      {rsaPrivateKey, false},
	"testdata/rsa.enc.priv.json":  {rsaPrivateKey, true},
}

var pemFiles = map[string]testdata{
	"../pemutil/testdata/openssl.p256.pem":              {ecdsaPrivateKey, false},
	"../pemutil/testdata/openssl.p256.pub.pem":          {ecdsaPublicKey, false},
	"../pemutil/testdata/openssl.p256.enc.pem":          {ecdsaPrivateKey, true},
	"../pemutil/testdata/openssl.p384.pem":              {ecdsaPrivateKey, false},
	"../pemutil/testdata/openssl.p384.pub.pem":          {ecdsaPublicKey, false},
	"../pemutil/testdata/openssl.p384.enc.pem":          {ecdsaPrivateKey, true},
	"../pemutil/testdata/openssl.p521.pem":              {ecdsaPrivateKey, false},
	"../pemutil/testdata/openssl.p521.pub.pem":          {ecdsaPublicKey, false},
	"../pemutil/testdata/openssl.p521.enc.pem":          {ecdsaPrivateKey, true},
	"../pemutil/testdata/openssl.rsa1024.pem":           {rsaPrivateKey, false},
	"../pemutil/testdata/openssl.rsa1024.pub.pem":       {rsaPublicKey, false},
	"../pemutil/testdata/openssl.rsa1024.enc.pem":       {rsaPrivateKey, true},
	"../pemutil/testdata/openssl.rsa2048.pem":           {rsaPrivateKey, false},
	"../pemutil/testdata/openssl.rsa2048.pub.pem":       {rsaPublicKey, false},
	"../pemutil/testdata/openssl.rsa2048.enc.pem":       {rsaPrivateKey, true},
	"../pemutil/testdata/pkcs8/openssl.ed25519.pem":     {ed25519PrivateKey, false},
	"../pemutil/testdata/pkcs8/openssl.ed25519.pub.pem": {ed25519PublicKey, false},
	"../pemutil/testdata/pkcs8/openssl.ed25519.enc.pem": {ed25519PrivateKey, true},
	"../pemutil/testdata/pkcs8/openssl.p256.pem":        {ecdsaPrivateKey, false},
	"../pemutil/testdata/pkcs8/openssl.p256.pub.pem":    {ecdsaPublicKey, false},
	"../pemutil/testdata/pkcs8/openssl.p256.enc.pem":    {ecdsaPrivateKey, true},
	"../pemutil/testdata/pkcs8/openssl.p384.pem":        {ecdsaPrivateKey, false},
	"../pemutil/testdata/pkcs8/openssl.p384.pub.pem":    {ecdsaPublicKey, false},
	"../pemutil/testdata/pkcs8/openssl.p384.enc.pem":    {ecdsaPrivateKey, true},
	"../pemutil/testdata/pkcs8/openssl.p521.pem":        {ecdsaPrivateKey, false},
	"../pemutil/testdata/pkcs8/openssl.p521.pub.pem":    {ecdsaPublicKey, false},
	"../pemutil/testdata/pkcs8/openssl.p521.enc.pem":    {ecdsaPrivateKey, true},
	"../pemutil/testdata/pkcs8/openssl.rsa2048.pem":     {rsaPrivateKey, false},
	"../pemutil/testdata/pkcs8/openssl.rsa2048.pub.pem": {rsaPublicKey, false},
	"../pemutil/testdata/pkcs8/openssl.rsa2048.enc.pem": {rsaPrivateKey, true},
	"../pemutil/testdata/pkcs8/openssl.rsa4096.pem":     {rsaPrivateKey, false},
	"../pemutil/testdata/pkcs8/openssl.rsa4096.pub.pem": {rsaPublicKey, false},
}

func validateReadKey(t *testing.T, fn, pass string, td testdata) {
	var err error
	var jwk *JSONWebKey

	if td.encrypted {
		jwk, err = ReadKey(fn, WithPassword([]byte(pass)))
	} else {
		jwk, err = ReadKey(fn)
	}
	assert.NoError(t, err)

	assert.NoError(t, ValidateJWK(jwk))

	switch td.typ {
	case ecdsaPublicKey:
		assert.Type(t, &ecdsa.PublicKey{}, jwk.Key)
	case ecdsaPrivateKey:
		assert.Type(t, &ecdsa.PrivateKey{}, jwk.Key)
	case ed25519PublicKey:
		assert.Type(t, ed25519.PublicKey{}, jwk.Key)
	case ed25519PrivateKey:
		assert.Type(t, ed25519.PrivateKey{}, jwk.Key)
	case rsaPublicKey:
		assert.Type(t, &rsa.PublicKey{}, jwk.Key)
	case rsaPrivateKey:
		assert.Type(t, &rsa.PrivateKey{}, jwk.Key)
	case octKey:
		assert.Type(t, []byte{}, jwk.Key)
	default:
		t.Errorf("type %T not supported", jwk.Key)
	}

	if jwk.IsPublic() == false && jwk.KeyID != "" {
		hash, err := jwk.Thumbprint(crypto.SHA256)
		assert.NoError(t, err)
		assert.Equals(t, base64.RawURLEncoding.EncodeToString(hash), jwk.KeyID)
	}

	if td.encrypted {
		jwkPriv, err := ReadKey(strings.Replace(fn, ".enc", "", 1))
		assert.NoError(t, err)
		assert.Equals(t, jwkPriv, jwk)
	}
}

func TestReadKey(t *testing.T) {
	for fn, td := range files {
		fn, td := fn, td
		t.Run(fn, func(t *testing.T) {
			validateReadKey(t, fn, "password", td)
		})
	}

	for fn, td := range pemFiles {
		fn, td := fn, td
		t.Run(fn, func(t *testing.T) {
			validateReadKey(t, fn, "mypassword", td)
		})
	}

	if _, err := ReadKey("testdata/missing.txt"); err == nil {
		t.Errorf("ReadKey() error = %v, wantErr true", err)
	}
}

func TestReadKey_https(t *testing.T) {
	ok, err := os.ReadFile("testdata/okp.pub.json")
	assert.FatalError(t, err)
	key, err := base64.RawURLEncoding.DecodeString("L4WYxHsMVaspyhWuSp84v2meEYMEUdYnrn-w-jqP6iw")
	assert.FatalError(t, err)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/ok":
			w.Header().Set("ContentType", "application/jwk-set+json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, string(ok))
		case "/empty":
			w.Header().Set("ContentType", "application/jwk-set+json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "{}")
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, http.StatusText(http.StatusNotFound))
		}
	}))
	srvClient := srv.Client()
	defer func() {
		srv.Close()
		http.DefaultClient = &http.Client{}
	}()

	type args struct {
		client   *http.Client
		filename string
		opts     []Option
	}
	tests := []struct {
		name    string
		args    args
		want    *JSONWebKey
		wantErr bool
	}{
		{"ok", args{srvClient, srv.URL + "/ok", nil}, &JSONWebKey{
			Key:                         ed25519.PublicKey(key),
			KeyID:                       "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM",
			Algorithm:                   "EdDSA",
			Use:                         "sig",
			Certificates:                []*x509.Certificate{},
			CertificateThumbprintSHA1:   []byte{},
			CertificateThumbprintSHA256: []byte{},
		}, false},
		{"failWithKid", args{srvClient, srv.URL + "/ok", []Option{WithKid("foobar")}}, nil, true},
		{"failEmpty", args{srvClient, srv.URL + "/empty", nil}, nil, true},
		{"failNotFound", args{srvClient, srv.URL + "/notFound", nil}, nil, true},
		{"failClient", args{&http.Client{}, srv.URL + "/ok", nil}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			http.DefaultClient = tt.args.client
			got, err := ReadKey(tt.args.filename, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadKeySet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadKeySet() = \n%#v, want \n%#v", got, tt.want)
			}
		})
	}
}

func TestReadKeyPasswordFile(t *testing.T) {
	jwk, err := ReadKey("testdata/oct.txt", WithAlg("HS256"), WithUse("sig"), WithKid("the-kid"))
	assert.FatalError(t, err)
	assert.Equals(t, []byte("a true random password"), jwk.Key)
	assert.Equals(t, HS256, jwk.Algorithm)
	assert.Equals(t, "sig", jwk.Use)
	assert.Equals(t, "the-kid", jwk.KeyID)
}

func TestParseKey(t *testing.T) {
	t.Parallel()
	marshal := func(i interface{}) []byte {
		b, err := json.Marshal(i)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	read := func(filename string) []byte {
		b, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	ecKey := fixJWK(mustGenerateJWK(t, "EC", "P-256", "ES256", "enc", "", 0))
	rsaKey := fixJWK(mustGenerateJWK(t, "RSA", "", "RS256", "sig", "", 2048))
	rsaPSSKey := fixJWK(mustGenerateJWK(t, "RSA", "", "PS256", "enc", "", 2048))
	edKey := fixJWK(mustGenerateJWK(t, "OKP", "Ed25519", "EdDSA", "sig", "", 0))
	octKey := fixJWK(mustGenerateJWK(t, "oct", "", "HS256", "sig", "", 64))

	encKey, err := EncryptJWK(edKey, testPassword)
	assert.FatalError(t, err)
	encKeyCompact, err := encKey.CompactSerialize()
	assert.FatalError(t, err)

	pemKey, err := pemutil.Read("../pemutil/testdata/pkcs8/openssl.ed25519.pem")
	assert.FatalError(t, err)

	ecKey.KeyID, err = Thumbprint(ecKey)
	assert.FatalError(t, err)

	type args struct {
		b    []byte
		opts []Option
	}
	tests := []struct {
		name    string
		args    args
		want    *JSONWebKey
		wantErr bool
	}{
		{"ec", args{marshal(ecKey), nil}, ecKey, false},
		{"rsa", args{marshal(rsaKey), nil}, rsaKey, false},
		{"rsa-pss", args{marshal(rsaPSSKey), nil}, rsaPSSKey, false},
		{"okp", args{marshal(edKey), nil}, edKey, false},
		{"oct", args{marshal(octKey), nil}, octKey, false},
		{"encryptedCompactWithPassword", args{[]byte(encKeyCompact), []Option{WithPassword(testPassword)}}, edKey, false},
		{"encryptedFullWithPasswordFile", args{[]byte(encKey.FullSerialize()), []Option{WithPasswordFile("testdata/passphrase.txt")}}, edKey, false},
		{"pemPrivate", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.pem"), nil}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"pemPublic", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.pub.pem"), nil}, &JSONWebKey{
			Key:       pemKey.(ed25519.PrivateKey).Public(),
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithPassword", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.enc.pem"), []Option{WithPassword([]byte("mypassword"))}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithPasswordFile", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.enc.pem"), []Option{WithPasswordFile("../pemutil/testdata/password.txt")}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithPasswordPrompter", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.enc.pem"), []Option{WithPasswordPrompter("What's the password", func(s string) ([]byte, error) {
			return []byte("mypassword"), nil
		})}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithKid", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.pem"), []Option{WithKid("foobarzar")}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "foobarzar",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithUse", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.pem"), []Option{WithUse("enc")}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Use:       "enc",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithAlg", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.pem"), []Option{WithAlg("EdDSA")}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"pemPrivateWithAlgWithSubtle", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.pem"), []Option{WithAlg("FOOBAR"), WithSubtle(true)}}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "FOOBAR",
		}, false},
		{"octPrivateWithAlg", args{testPassword, []Option{WithAlg("HS256")}}, &JSONWebKey{
			Key:       testPassword,
			Algorithm: "HS256",
		}, false},
		{"octPrivateWithAlgWithKid", args{testPassword, []Option{WithAlg("HS256"), WithKid("foobarzar")}}, &JSONWebKey{
			Key:       testPassword,
			KeyID:     "foobarzar",
			Algorithm: "HS256",
		}, false},
		{"failPassword", args{[]byte(encKeyCompact), []Option{WithPassword([]byte("bad password"))}}, nil, true},
		{"failMissingFile", args{[]byte(encKeyCompact), []Option{WithPasswordFile("testdata/missing.txt")}}, nil, true},
		{"failPEMPassword", args{read("../pemutil/testdata/pkcs8/openssl.ed25519.enc.pem"), []Option{WithPassword([]byte("bad password"))}}, nil, true},
		{"failECBWongAlg", args{marshal(ecKey), []Option{WithAlg("FOOBAR")}}, nil, true},
		{"failECWrongKid", args{marshal(ecKey), []Option{WithKid("foobarzar")}}, nil, true},
		{"failOCTMissingOptions", args{testPassword, nil}, nil, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseKey(tt.args.b, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Make the rsa keys equal if they are
			if tt.want != nil {
				if k, ok := tt.want.Key.(*rsa.PrivateKey); ok {
					if !rsaEqual(k, got.Key) {
						t.Errorf("ParseKey() got = %v, want %v", got, tt.want)
						return
					}
					got.Key = k
				}
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseKeyPemutilPromptPassword(t *testing.T) {
	pemKey, err := pemutil.Read("../pemutil/testdata/pkcs8/openssl.ed25519.pem")
	assert.FatalError(t, err)

	pemBytes, err := os.ReadFile("../pemutil/testdata/pkcs8/openssl.ed25519.enc.pem")
	assert.FatalError(t, err)

	tmp0 := pemutil.PromptPassword
	tmp1 := PromptPassword
	t.Cleanup(func() {
		pemutil.PromptPassword = tmp0
		PromptPassword = tmp1
	})

	tests := []struct {
		name           string
		promptPassword PasswordPrompter
		want           *JSONWebKey
		wantErr        bool
	}{
		{"ok", func(s string) ([]byte, error) {
			return []byte("mypassword"), nil
		}, &JSONWebKey{
			Key:       pemKey,
			KeyID:     "vEk4UARa85PrW0eea2zeVLqGBF-n5Jzd9GVmKAc0AHQ",
			Algorithm: "EdDSA",
		}, false},
		{"fail", func(s string) ([]byte, error) {
			return []byte("not-mypassword"), nil
		}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PromptPassword = tt.promptPassword
			pemutil.PromptPassword = nil
			got, err := ParseKey(pemBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadKeySet(t *testing.T) {
	jwk, err := ReadKeySet("testdata/jwks.json", WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM"))
	assert.NoError(t, err)
	assert.Type(t, ed25519.PublicKey{}, jwk.Key)
	assert.Equals(t, "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM", jwk.KeyID)

	jwk, err = ReadKeySet("testdata/jwks.json", WithKid("V93A-Yh7Bhw1W2E0igFciviJzX4PXPswoVgriehm9Co"))
	assert.NoError(t, err)
	assert.Type(t, &ecdsa.PublicKey{}, jwk.Key)
	assert.Equals(t, "V93A-Yh7Bhw1W2E0igFciviJzX4PXPswoVgriehm9Co", jwk.KeyID)

	jwk, err = ReadKeySet("testdata/jwks.json", WithKid("duplicated"))
	assert.Error(t, err)
	assert.Equals(t, "multiple keys with kid duplicated have been found on testdata/jwks.json", err.Error())
	assert.Nil(t, jwk)

	jwk, err = ReadKeySet("testdata/jwks.json", WithKid("missing"))
	assert.Error(t, err)
	assert.Equals(t, "cannot find key with kid missing on testdata/jwks.json", err.Error())
	assert.Nil(t, jwk)

	jwk, err = ReadKeySet("testdata/empty.json", WithKid("missing"))
	assert.Error(t, err)
	assert.Equals(t, "cannot find key with kid missing on testdata/empty.json", err.Error())
	assert.Nil(t, jwk)
}

func TestReadKeySet_https(t *testing.T) {
	ok, err := os.ReadFile("testdata/jwks.json")
	assert.FatalError(t, err)
	key, err := base64.RawURLEncoding.DecodeString("L4WYxHsMVaspyhWuSp84v2meEYMEUdYnrn-w-jqP6iw")
	assert.FatalError(t, err)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/ok":
			w.Header().Set("ContentType", "application/jwk-set+json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, string(ok))
		case "/empty":
			w.Header().Set("ContentType", "application/jwk-set+json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "{}")
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, http.StatusText(http.StatusNotFound))
		}
	}))
	srvClient := srv.Client()
	defer func() {
		srv.Close()
		http.DefaultClient = &http.Client{}
	}()

	type args struct {
		client   *http.Client
		filename string
		opts     []Option
	}
	tests := []struct {
		name    string
		args    args
		want    *JSONWebKey
		wantErr bool
	}{
		{"ok", args{srvClient, srv.URL + "/ok", []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM")}}, &JSONWebKey{
			Key:                         ed25519.PublicKey(key),
			KeyID:                       "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM",
			Algorithm:                   "EdDSA",
			Use:                         "sig",
			Certificates:                []*x509.Certificate{},
			CertificateThumbprintSHA1:   []byte{},
			CertificateThumbprintSHA256: []byte{},
		}, false},
		{"failEmpty", args{srvClient, srv.URL + "/empty", []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM")}}, nil, true},
		{"failNotFound", args{srvClient, srv.URL + "/notFound", []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM")}}, nil, true},
		{"failClient", args{&http.Client{}, srv.URL + "/ok", []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM")}}, nil, true},
		{"failNoOptions", args{srvClient, srv.URL + "/ok", nil}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			http.DefaultClient = tt.args.client
			got, err := ReadKeySet(tt.args.filename, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadKeySet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadKeySet() = \n%#v, want \n%#v", got, tt.want)
			}
		})
	}
}

func TestGuessJWKAlgorithm(t *testing.T) {
	p256, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.FatalError(t, err)
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	assert.FatalError(t, err)
	p521, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	assert.FatalError(t, err)
	rsa2k, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.FatalError(t, err)
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	assert.FatalError(t, err)
	xPub, xPriv, err := x25519.GenerateKey(rand.Reader)
	assert.FatalError(t, err)

	tests := []struct {
		jwk      *JSONWebKey
		expected string
	}{
		{&JSONWebKey{Key: []byte{}, Use: ""}, HS256},
		{&JSONWebKey{Key: []byte{}, Use: "sig"}, HS256},
		{&JSONWebKey{Key: []byte{}, Use: "enc"}, "A256GCMKW"},
		{&JSONWebKey{Key: p256, Use: ""}, ES256},
		{&JSONWebKey{Key: p384, Use: "sig"}, ES384},
		{&JSONWebKey{Key: p521, Use: "enc"}, "ECDH-ES"},
		{&JSONWebKey{Key: p256.Public(), Use: ""}, ES256},
		{&JSONWebKey{Key: p384.Public(), Use: "sig"}, ES384},
		{&JSONWebKey{Key: p521.Public(), Use: "enc"}, "ECDH-ES"},
		{&JSONWebKey{Key: rsa2k, Use: ""}, RS256},
		{&JSONWebKey{Key: rsa2k, Use: "sig"}, RS256},
		{&JSONWebKey{Key: rsa2k, Use: "enc"}, "RSA-OAEP-256"},
		{&JSONWebKey{Key: rsa2k.Public(), Use: ""}, RS256},
		{&JSONWebKey{Key: rsa2k.Public(), Use: "sig"}, RS256},
		{&JSONWebKey{Key: rsa2k.Public(), Use: "enc"}, "RSA-OAEP-256"},
		{&JSONWebKey{Key: edPub, Use: ""}, EdDSA},
		{&JSONWebKey{Key: edPub, Use: "sig"}, EdDSA},
		{&JSONWebKey{Key: edPriv, Use: ""}, EdDSA},
		{&JSONWebKey{Key: edPriv, Use: "sig"}, EdDSA},
		{&JSONWebKey{Key: xPub, Use: ""}, XEdDSA},
		{&JSONWebKey{Key: xPub, Use: "sig"}, XEdDSA},
		{&JSONWebKey{Key: xPriv, Use: ""}, XEdDSA},
		{&JSONWebKey{Key: xPriv, Use: "sig"}, XEdDSA},
	}

	// With context
	ctx, err := new(context).apply(WithAlg(HS256))
	assert.NoError(t, err)
	jwk := &JSONWebKey{Key: []byte("password")}
	guessJWKAlgorithm(ctx, jwk)
	assert.Equals(t, HS256, jwk.Algorithm)

	// With algorithm set
	ctx, err = new(context).apply(WithAlg(HS256))
	assert.NoError(t, err)
	jwk = &JSONWebKey{Key: []byte("password"), Algorithm: HS384}
	guessJWKAlgorithm(ctx, jwk)
	assert.Equals(t, HS384, jwk.Algorithm)

	// With no defaults
	ctx, err = new(context).apply(WithNoDefaults(true))
	assert.NoError(t, err)
	jwk = mustGenerateJWK(t, "EC", "P-256", "ES256", "sig", "", 0)
	jwk.Algorithm = ""
	guessJWKAlgorithm(ctx, jwk)
	assert.Equals(t, ES256, jwk.Algorithm)

	pub := jwk.Public()
	pub.Algorithm = ""
	guessJWKAlgorithm(ctx, &pub)
	assert.Equals(t, ES256, pub.Algorithm)

	jwk = mustGenerateJWK(t, "OKP", "Ed25519", "EdDSA", "sig", "", 0)
	jwk.Algorithm = ""
	guessJWKAlgorithm(ctx, jwk)
	assert.Equals(t, EdDSA, jwk.Algorithm)

	pub = jwk.Public()
	pub.Algorithm = ""
	guessJWKAlgorithm(ctx, &pub)
	assert.Equals(t, EdDSA, pub.Algorithm)

	jwk = &JSONWebKey{Key: xPriv, Algorithm: "", Use: "sig"}
	guessJWKAlgorithm(ctx, jwk)
	assert.Equals(t, XEdDSA, jwk.Algorithm)

	jwk = &JSONWebKey{Key: xPub, Algorithm: "", Use: "sig"}
	guessJWKAlgorithm(ctx, jwk)
	assert.Equals(t, XEdDSA, jwk.Algorithm)

	// Defaults
	for _, tc := range tests {
		guessJWKAlgorithm(new(context), tc.jwk)
		assert.Equals(t, tc.expected, tc.jwk.Algorithm)
	}
}

func TestParseKeySet(t *testing.T) {
	b, err := os.ReadFile("testdata/jwks.json")
	assert.FatalError(t, err)
	key, err := base64.RawURLEncoding.DecodeString("L4WYxHsMVaspyhWuSp84v2meEYMEUdYnrn-w-jqP6iw")
	assert.FatalError(t, err)

	jwe := mustEncryptData(t, b, testPassword)
	encryptedJSON := []byte(jwe.FullSerialize())
	encryptedCompact, err := jwe.CompactSerialize()
	assert.FatalError(t, err)

	type args struct {
		b    []byte
		opts []Option
	}
	tests := []struct {
		name    string
		args    args
		want    *JSONWebKey
		wantErr bool
	}{
		{"ok", args{b, []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM")}}, &JSONWebKey{
			Key:                         ed25519.PublicKey(key),
			KeyID:                       "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM",
			Algorithm:                   "EdDSA",
			Use:                         "sig",
			Certificates:                []*x509.Certificate{},
			CertificateThumbprintSHA1:   []byte{},
			CertificateThumbprintSHA256: []byte{},
		}, false},
		{"okEncryptedJSON", args{encryptedJSON, []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM"), WithPassword(testPassword)}}, &JSONWebKey{
			Key:                         ed25519.PublicKey(key),
			KeyID:                       "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM",
			Algorithm:                   "EdDSA",
			Use:                         "sig",
			Certificates:                []*x509.Certificate{},
			CertificateThumbprintSHA1:   []byte{},
			CertificateThumbprintSHA256: []byte{},
		}, false},
		{"okEncryptedCompact", args{[]byte(encryptedCompact), []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM"), WithPasswordFile("testdata/passphrase.txt")}}, &JSONWebKey{
			Key:                         ed25519.PublicKey(key),
			KeyID:                       "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM",
			Algorithm:                   "EdDSA",
			Use:                         "sig",
			Certificates:                []*x509.Certificate{},
			CertificateThumbprintSHA1:   []byte{},
			CertificateThumbprintSHA256: []byte{},
		}, false},
		{"okWithAlgSubtle", args{b, []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM"), WithSubtle(true), WithAlg("FOOBAR")}}, &JSONWebKey{
			Key:                         ed25519.PublicKey(key),
			KeyID:                       "qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM",
			Algorithm:                   "FOOBAR",
			Use:                         "sig",
			Certificates:                []*x509.Certificate{},
			CertificateThumbprintSHA1:   []byte{},
			CertificateThumbprintSHA256: []byte{},
		}, false},
		{"failOptions", args{b, []Option{WithPasswordFile("testdata/missing.txt")}}, nil, true},
		{"failDecrypt", args{encryptedJSON, []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM"), WithPassword([]byte("bad-password"))}}, nil, true},
		{"failNoOptions", args{b, []Option{}}, nil, true},
		{"failBadData", args{[]byte("foo"), []Option{}}, nil, true},
		{"failEmpty", args{[]byte("{}"), []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM")}}, nil, true},
		{"failDuplicated", args{b, []Option{WithKid("duplicated")}}, nil, true},
		{"failWithAlg", args{b, []Option{WithKid("qiCJG7r2L80rmWRrZMPfpanQHmZRcncOG7A7MBWn9qM"), WithAlg("FOOBAR")}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKeySet(tt.args.b, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeySet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseKeySet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_guessKeyType(t *testing.T) {
	marshal := func(i interface{}) []byte {
		b, err := json.Marshal(i)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	ecKey := mustGenerateJWK(t, "EC", "P-256", "ES256", "enc", "", 0)
	rsaKey := mustGenerateJWK(t, "RSA", "", "RS256", "sig", "", 2048)
	rsaPSSKey := mustGenerateJWK(t, "RSA", "", "PS256", "enc", "", 2048)
	edKey := mustGenerateJWK(t, "OKP", "Ed25519", "EdDSA", "sig", "", 0)
	octKey := mustGenerateJWK(t, "oct", "", "HS256", "sig", "", 64)
	octHS384 := mustGenerateJWK(t, "oct", "", "HS384", "sig", "", 64)

	rsaPEM, err := os.ReadFile("../pemutil/testdata/openssl.p256.pem")
	assert.FatalError(t, err)

	type args struct {
		ctx  *context
		data []byte
	}
	tests := []struct {
		name string
		args args
		want keyType
	}{
		{"ec", args{&context{}, marshal(ecKey)}, jwkKeyType},
		{"rsaKey", args{&context{}, marshal(rsaKey)}, jwkKeyType},
		{"rsaPSSKey", args{&context{}, marshal(rsaPSSKey)}, jwkKeyType},
		{"edKey", args{&context{}, marshal(edKey)}, jwkKeyType},
		{"octKey", args{&context{}, marshal(octKey)}, jwkKeyType},
		{"encrypted", args{&context{}, marshal(mustEncryptJWK(t, ecKey, testPassword))}, jwkKeyType},
		{"rsaPEM", args{&context{}, rsaPEM}, pemKeyType},
		{"encryptedAlgHS256", args{&context{alg: "HS256"}, []byte(mustEncryptJWK(t, octKey, testPassword).FullSerialize())}, jwkKeyType},
		{"jwkAlgHS384", args{&context{alg: "HS384"}, marshal(octHS384)}, jwkKeyType},
		{"bloblAlgHS512", args{&context{alg: "HS512"}, testPassword}, octKeyType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guessKeyType(tt.args.ctx, tt.args.data); got != tt.want {
				t.Errorf("guessKeyType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_guessSignatureAlgorithm(t *testing.T) {
	must := func(args ...interface{}) crypto.PrivateKey {
		last := len(args) - 1
		if err := args[last]; err != nil {
			t.Fatal(err)
		}
		return args[last-1]
	}

	p256, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)
	p521, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	require.NoError(t, err)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, x25519Key, err := x25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	type args struct {
		key crypto.PrivateKey
	}
	tests := []struct {
		name string
		args args
		want SignatureAlgorithm
	}{
		{"byte", args{[]byte("the-key")}, HS256},
		{"ES256", args{p256}, ES256},
		{"ES384", args{p384}, ES384},
		{"ES512", args{p521}, ES512},
		{"RS256", args{rsaKey}, RS256},
		{"EdDSA", args{edKey}, EdDSA},
		{"XEdDSA", args{x25519Key}, XEdDSA},
		{"XEdDSA with X25519Signer", args{X25519Signer(x25519Key)}, XEdDSA},
		{"signer ES256", args{wrapSigner{p256}}, ES256},
		{"signer ES384", args{wrapSigner{p384}}, ES384},
		{"signer ES512", args{wrapSigner{p521}}, ES512},
		{"signer RS256", args{wrapSigner{rsaKey}}, RS256},
		{"signer EdDSA", args{wrapSigner{edKey}}, EdDSA},
		{"signer XEdDSA", args{wrapSigner{x25519Key}}, XEdDSA},
		{"opaque ES256", args{NewOpaqueSigner(p256)}, ES256},
		{"opaque ES384", args{NewOpaqueSigner(p384)}, ES384},
		{"opaque ES512", args{NewOpaqueSigner(p521)}, ES512},
		{"opaque RS256", args{NewOpaqueSigner(rsaKey)}, RS256},
		{"opaque EdDSA", args{NewOpaqueSigner(edKey)}, EdDSA},
		{"opaque XEdDSA", args{NewOpaqueSigner(x25519Key)}, XEdDSA},
		{"empty", args{must(ecdsa.GenerateKey(elliptic.P224(), rand.Reader))}, ""},
		{"signer empty", args{wrapSigner{}}, ""},
		{"opaque empty", args{NewOpaqueSigner(wrapSigner{})}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guessSignatureAlgorithm(tt.args.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("guessSignatureAlgorithm() = %v, want %v", got, tt.want)
			}
		})
	}
}
