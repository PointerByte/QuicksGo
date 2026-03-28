// Copyright 2026 PointerByte Contributors
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/PointerByte/QuicksGo/security/encrypt/common"
	"github.com/zeebo/blake3"
)

func TestNewRepositoryBuildsAllRepositories(t *testing.T) {
	repository := NewRepository()
	if repository.SymmetricRepository == nil || repository.AsymmetricRepository == nil || repository.SignatureRepository == nil || repository.HashRepository == nil {
		t.Fatal("expected all repositories to be initialized")
	}
}

func TestSymmetricRepositoryAESAndFernet(t *testing.T) {
	repository := NewSymmetricRepository()

	key, err := repository.GeneratesSymetrycKey(common.Key256Bits)
	if err != nil {
		t.Fatalf("GeneratesSymetrycKey() error = %v", err)
	}
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	if len(keyBytes) != int(common.Key256Bits) {
		t.Fatalf("key length = %d, want %d", len(keyBytes), common.Key256Bits)
	}

	ciphertext, err := repository.EncryptAES(key, "hello", "aad")
	if err != nil {
		t.Fatalf("EncryptAES() error = %v", err)
	}
	plaintext, err := repository.DecryptAES(key, ciphertext, "aad")
	if err != nil {
		t.Fatalf("DecryptAES() error = %v", err)
	}
	if plaintext != "hello" {
		t.Fatalf("DecryptAES() = %q, want %q", plaintext, "hello")
	}

	fernetKeyBytes := make([]byte, 32)
	if _, err := rand.Read(fernetKeyBytes); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	fernetKey := base64.StdEncoding.EncodeToString(fernetKeyBytes)
	token, err := repository.EncodeFernet(fernetKey, "payload")
	if err != nil {
		t.Fatalf("EncodeFernet() error = %v", err)
	}
	decoded, err := repository.DecodeFernet(fernetKey, token)
	if err != nil {
		t.Fatalf("DecodeFernet() error = %v", err)
	}
	if decoded != "payload" {
		t.Fatalf("DecodeFernet() = %q, want %q", decoded, "payload")
	}
}

func TestSymmetricRepositoryErrors(t *testing.T) {
	repository := NewSymmetricRepository()

	if _, err := repository.EncryptAES("%%%", "value", "aad"); err == nil {
		t.Fatal("expected EncryptAES() base64 error")
	}
	if _, err := repository.EncryptAES(base64.StdEncoding.EncodeToString([]byte("short")), "value", "aad"); err == nil {
		t.Fatal("expected EncryptAES() invalid key error")
	}
	if _, err := repository.DecryptAES("%%%", "cipher", "aad"); err == nil {
		t.Fatal("expected DecryptAES() key error")
	}

	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if _, err := repository.DecryptAES(key, "%%%", "aad"); err == nil {
		t.Fatal("expected DecryptAES() ciphertext error")
	}
	if _, err := repository.DecryptAES(key, base64.StdEncoding.EncodeToString([]byte("short")), "aad"); err == nil {
		t.Fatal("expected DecryptAES() short ciphertext error")
	}

	ciphertext, err := repository.EncryptAES(key, "hello", "aad")
	if err != nil {
		t.Fatalf("EncryptAES() error = %v", err)
	}
	if _, err := repository.DecryptAES(key, ciphertext, "wrong"); err == nil {
		t.Fatal("expected DecryptAES() authentication error")
	}

	if _, err := decodeFernetKey("%%%"); err == nil {
		t.Fatal("expected decodeFernetKey() base64 error")
	}
	if _, err := decodeFernetKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("expected decodeFernetKey() length error")
	}
	if _, err := repository.EncodeFernet("%%%", "payload"); err == nil {
		t.Fatal("expected EncodeFernet() key error")
	}
	if _, err := repository.DecodeFernet("%%%", "payload"); err == nil {
		t.Fatal("expected DecodeFernet() key error")
	}
	if _, err := repository.DecodeFernet(base64.StdEncoding.EncodeToString(make([]byte, 32)), "%%%"); err == nil {
		t.Fatal("expected DecodeFernet() token decode error")
	}
	if _, err := repository.DecodeFernet(base64.StdEncoding.EncodeToString(make([]byte, 32)), base64.URLEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("expected DecodeFernet() short token error")
	}

	fernetKeyBytes := make([]byte, 32)
	if _, err := rand.Read(fernetKeyBytes); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	fernetKey := base64.StdEncoding.EncodeToString(fernetKeyBytes)
	token, err := repository.EncodeFernet(fernetKey, "payload")
	if err != nil {
		t.Fatalf("EncodeFernet() error = %v", err)
	}

	rawToken, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	rawToken[len(rawToken)-1] ^= 0xff
	if _, err := repository.DecodeFernet(fernetKey, base64.URLEncoding.EncodeToString(rawToken)); err == nil {
		t.Fatal("expected DecodeFernet() invalid HMAC error")
	}

	keyInfo, err := decodeFernetKey(fernetKey)
	if err != nil {
		t.Fatalf("decodeFernetKey() error = %v", err)
	}
	rawToken, err = base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	message := rawToken[:len(rawToken)-32]
	message = append(message[:len(message)-1], message[len(message):]...)
	mac := hmac.New(sha256.New, keyInfo.signingKey)
	mac.Write(message)
	invalidLengthToken := append(message, mac.Sum(nil)...)
	if _, err := repository.DecodeFernet(fernetKey, base64.URLEncoding.EncodeToString(invalidLengthToken)); err == nil {
		t.Fatal("expected DecodeFernet() block-size error")
	}
}

func TestHashRepository(t *testing.T) {
	repository := NewHashRepository()

	got := repository.GenerateHMAC("message", "secret")
	if got == "" {
		t.Fatal("GenerateHMAC() returned empty value")
	}
	if !repository.ValidateHMAC("message", "secret", got) {
		t.Fatal("ValidateHMAC() = false, want true")
	}
	if repository.ValidateHMAC("message", "secret", "bad") {
		t.Fatal("ValidateHMAC() = true, want false")
	}

	wantSHA := hex.EncodeToString(mustSHA256Bytes([]byte("message")))
	if got := repository.Sha256Hex("message"); got != wantSHA {
		t.Fatalf("Sha256Hex() = %q, want %q", got, wantSHA)
	}

	blakeSum := blake3.Sum256([]byte("message"))
	wantBlake := base64.StdEncoding.EncodeToString(blakeSum[:])
	if got := repository.Blake3("message"); got != wantBlake {
		t.Fatalf("Blake3() = %q, want %q", got, wantBlake)
	}
}

func TestAsymmetricAndSignatureRepositories(t *testing.T) {
	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	priv, pub, err := asymmetricRepository.GeneratesRSAKey(common.Key2048Bits)
	if err != nil {
		t.Fatalf("GeneratesRSAKey() error = %v", err)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(mustBase64Decode(t, priv))
	if err != nil {
		t.Fatalf("ParsePKCS1PrivateKey() error = %v", err)
	}
	publicKey, err := x509.ParsePKCS1PublicKey(mustBase64Decode(t, pub))
	if err != nil {
		t.Fatalf("ParsePKCS1PublicKey() error = %v", err)
	}

	ciphertext, err := asymmetricRepository.RSA_OAEP_Encode(mustMarshalPKIXRSAPublicKey(t, publicKey), "hello")
	if err != nil {
		t.Fatalf("RSA_OAEP_Encode() error = %v", err)
	}
	plaintext, err := asymmetricRepository.RSA_OAEP_Decode(mustMarshalPKCS8RSAPrivateKey(t, privateKey), ciphertext)
	if err != nil {
		t.Fatalf("RSA_OAEP_Decode() error = %v", err)
	}
	if plaintext != "hello" {
		t.Fatalf("RSA_OAEP_Decode() = %q, want %q", plaintext, "hello")
	}

	signature, err := signatureRepository.SignRSAPSS(mustMarshalPKCS8RSAPrivateKey(t, privateKey), "payload")
	if err != nil {
		t.Fatalf("SignRSAPSS() error = %v", err)
	}
	if err := signatureRepository.VerifyRSAPSS(mustMarshalPKIXRSAPublicKey(t, publicKey), "payload", signature); err != nil {
		t.Fatalf("VerifyRSAPSS() error = %v", err)
	}

	pkcs1v15Signature, err := signatureRepository.SignSHA256("payload", privateKey)
	if err != nil {
		t.Fatalf("SignSHA256() error = %v", err)
	}
	if err := signatureRepository.VerifySHA256("payload", pkcs1v15Signature, publicKey); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
	}

	edPrivate, edPublic, err := signatureRepository.GeneratesEd255Key(common.Key2048Bits)
	if err != nil {
		t.Fatalf("GeneratesEd255Key() error = %v", err)
	}
	edSignature, err := signatureRepository.SignEd25519(edPrivate, "payload")
	if err != nil {
		t.Fatalf("SignEd25519() error = %v", err)
	}
	if err := signatureRepository.VerifyEd25519(edPublic, "payload", edSignature); err != nil {
		t.Fatalf("VerifyEd25519() error = %v", err)
	}

	if err := signatureRepository.VerifyEd25519(edPublic, "payload", edSignature[:len(edSignature)-2]+"ab"); err == nil {
		t.Fatal("expected VerifyEd25519() invalid signature error")
	}

	if err := signatureRepository.VerifyRSAPSS(mustMarshalPKIXRSAPublicKey(t, publicKey), "payload", signature[:len(signature)-2]+"ab"); err == nil {
		t.Fatal("expected VerifyRSAPSS() invalid signature error")
	}

	if err := signatureRepository.VerifySHA256("payload", pkcs1v15Signature[:len(pkcs1v15Signature)-2]+"ab", publicKey); err == nil {
		t.Fatal("expected VerifySHA256() invalid signature error")
	}
}

func TestAsymmetricAndSignatureRepositoryErrors(t *testing.T) {
	asymmetricRepository := NewAsymmetricRepository()
	signatureRepository := NewSignatureRepository()

	if _, err := asymmetricRepository.RSA_OAEP_Encode("%%%", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Encode() key error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode("%%%", "payload"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() key error")
	}
	if _, err := asymmetricRepository.RSA_OAEP_Decode(mustMarshalPKCS8RSAPrivateKey(t, mustRSAKey(t)), "%%%"); err == nil {
		t.Fatal("expected RSA_OAEP_Decode() ciphertext error")
	}
	if _, _, err := asymmetricRepository.GeneratesRSAKey(0); err == nil {
		t.Fatal("expected GeneratesRSAKey() error")
	}

	if _, err := signatureRepository.SignEd25519("%%%", "payload"); err == nil {
		t.Fatal("expected SignEd25519() key error")
	}
	if err := signatureRepository.VerifyEd25519("%%%", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyEd25519() key error")
	}

	edPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}
	if err := signatureRepository.VerifyEd25519(mustMarshalEd25519PublicKey(t, edPublic), "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyEd25519() signature decode error")
	}

	if _, err := signatureRepository.SignRSAPSS("%%%", "payload"); err == nil {
		t.Fatal("expected SignRSAPSS() key error")
	}
	if err := signatureRepository.VerifyRSAPSS("%%%", "payload", "sig"); err == nil {
		t.Fatal("expected VerifyRSAPSS() key error")
	}
	if err := signatureRepository.VerifyRSAPSS(mustMarshalPKIXRSAPublicKey(t, &mustRSAKey(t).PublicKey), "payload", "%%%"); err == nil {
		t.Fatal("expected VerifyRSAPSS() signature decode error")
	}

	if _, err := signatureRepository.SignSHA256("payload", nil); err == nil {
		t.Fatal("expected SignSHA256() nil private key error")
	}
	if err := signatureRepository.VerifySHA256("payload", "sig", nil); err == nil {
		t.Fatal("expected VerifySHA256() nil public key error")
	}
	if err := signatureRepository.VerifySHA256("payload", "%%%", &mustRSAKey(t).PublicKey); err == nil {
		t.Fatal("expected VerifySHA256() signature decode error")
	}
}

func TestPKCS7Helpers(t *testing.T) {
	padded := pkcs7Pad([]byte("abc"), 4)
	if len(padded)%4 != 0 {
		t.Fatal("pkcs7Pad() length should align to block size")
	}

	unpadded, err := pkcs7Unpad(padded, 4)
	if err != nil {
		t.Fatalf("pkcs7Unpad() error = %v", err)
	}
	if string(unpadded) != "abc" {
		t.Fatalf("pkcs7Unpad() = %q, want %q", string(unpadded), "abc")
	}

	if got := bytesRepeat('x', 3); string(got) != "xxx" {
		t.Fatalf("bytesRepeat() = %q, want %q", string(got), "xxx")
	}

	assertPanic(t, func() { pkcs7Pad([]byte("abc"), 0) })
	if _, err := pkcs7Unpad(nil, 4); err == nil {
		t.Fatal("expected pkcs7Unpad() size error")
	}
	if _, err := pkcs7Unpad([]byte{1, 2, 0}, 3); err == nil {
		t.Fatal("expected pkcs7Unpad() length error")
	}
	if _, err := pkcs7Unpad([]byte{1, 2, 4, 4}, 3); err == nil {
		t.Fatal("expected pkcs7Unpad() length > block error")
	}
	if _, err := pkcs7Unpad([]byte{1, 2, 3, 2}, 4); err == nil {
		t.Fatal("expected pkcs7Unpad() content error")
	}
}

func TestParseKeyUtilities(t *testing.T) {
	privateKey := mustRSAKey(t)
	publicKey := &privateKey.PublicKey
	edPublic, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

	if _, err := ParseRSAPublicKeyFromBase64(mustMarshalPKIXRSAPublicKey(t, publicKey)); err != nil {
		t.Fatalf("ParseRSAPublicKeyFromBase64() error = %v", err)
	}
	if _, err := ParseRSAPrivateKeyFromBase64(mustMarshalPKCS8RSAPrivateKey(t, privateKey)); err != nil {
		t.Fatalf("ParseRSAPrivateKeyFromBase64() error = %v", err)
	}
	if _, err := ParseEd25519PublicKeyFromBase64(mustMarshalEd25519PublicKey(t, edPublic)); err != nil {
		t.Fatalf("ParseEd25519PublicKeyFromBase64() error = %v", err)
	}
	if _, err := ParseEd25519PrivateKeyFromBase64(mustMarshalEd25519PrivateKey(t, edPrivate)); err != nil {
		t.Fatalf("ParseEd25519PrivateKeyFromBase64() error = %v", err)
	}

	if _, err := ParseRSAPublicKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseRSAPublicKeyFromBase64() error")
	}
	if _, err := ParseRSAPrivateKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseRSAPrivateKeyFromBase64() error")
	}
	if _, err := ParseEd25519PublicKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseEd25519PublicKeyFromBase64() error")
	}
	if _, err := ParseEd25519PrivateKeyFromBase64("%%%"); err == nil {
		t.Fatal("expected ParseEd25519PrivateKeyFromBase64() error")
	}

	rsaPublicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	rsaPrivateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	edPublicDER, err := x509.MarshalPKIXPublicKey(edPublic)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	edPrivateDER, err := x509.MarshalPKCS8PrivateKey(edPrivate)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}

	if _, err := ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseRSAPublicKeyFromBase64() parse error")
	}
	if _, err := ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseRSAPrivateKeyFromBase64() parse error")
	}
	if _, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseEd25519PublicKeyFromBase64() parse error")
	}
	if _, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatal("expected ParseEd25519PrivateKeyFromBase64() parse error")
	}

	if _, err := ParseRSAPublicKeyFromBase64(base64.StdEncoding.EncodeToString(edPublicDER)); err == nil {
		t.Fatal("expected ParseRSAPublicKeyFromBase64() wrong type error")
	}
	if _, err := ParseRSAPrivateKeyFromBase64(base64.StdEncoding.EncodeToString(edPrivateDER)); err == nil {
		t.Fatal("expected ParseRSAPrivateKeyFromBase64() wrong type error")
	}
	if _, err := ParseEd25519PublicKeyFromBase64(base64.StdEncoding.EncodeToString(rsaPublicDER)); err == nil {
		t.Fatal("expected ParseEd25519PublicKeyFromBase64() wrong type error")
	}
	if _, err := ParseEd25519PrivateKeyFromBase64(base64.StdEncoding.EncodeToString(rsaPrivateDER)); err == nil {
		t.Fatal("expected ParseEd25519PrivateKeyFromBase64() wrong type error")
	}
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return privateKey
}

func mustMarshalPKCS8RSAPrivateKey(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalPKIXRSAPublicKey(t *testing.T, publicKey *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalEd25519PrivateKey(t *testing.T, privateKey ed25519.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustMarshalEd25519PublicKey(t *testing.T, publicKey ed25519.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

func mustBase64Decode(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	return decoded
}

func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func mustSHA256Bytes(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
