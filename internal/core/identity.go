package core

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Identity struct {
	ID         string `json:"id"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}
type SignedEnvelope struct {
	Sender    string          `json:"sender"`
	Timestamp time.Time       `json:"timestamp"`
	Nonce     string          `json:"nonce"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

func LoadOrCreateIdentity(path string) (Identity, error) {
	if b, err := os.ReadFile(path); err == nil {
		var i Identity
		return i, json.Unmarshal(b, &i)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	h := sha256.Sum256(pub)
	i := Identity{ID: hex.EncodeToString(h[:8]), PublicKey: base64.RawStdEncoding.EncodeToString(pub), PrivateKey: base64.RawStdEncoding.EncodeToString(priv)}
	b, _ := json.MarshalIndent(i, "", "  ")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Identity{}, err
	}
	return i, os.WriteFile(path, b, 0o600)
}

func Sign(i Identity, payload []byte) (SignedEnvelope, error) {
	priv, err := base64.RawStdEncoding.DecodeString(i.PrivateKey)
	if err != nil {
		return SignedEnvelope{}, err
	}
	nonceBytes := make([]byte, 18)
	if _, err := rand.Read(nonceBytes); err != nil {
		return SignedEnvelope{}, err
	}
	e := SignedEnvelope{Sender: i.ID, Timestamp: time.Now().UTC(), Nonce: base64.RawStdEncoding.EncodeToString(nonceBytes), Payload: payload}
	e.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, signingBytes(e)))
	return e, nil
}

func Verify(e SignedEnvelope, publicKey string, now time.Time) error {
	if e.Timestamp.Before(now.Add(-2*time.Minute)) || e.Timestamp.After(now.Add(2*time.Minute)) {
		return errors.New("stale federation message")
	}
	pub, err := base64.RawStdEncoding.DecodeString(publicKey)
	if err != nil {
		return err
	}
	sig, err := base64.RawStdEncoding.DecodeString(e.Signature)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, signingBytes(e), sig) {
		return errors.New("invalid federation signature")
	}
	return nil
}

func signingBytes(e SignedEnvelope) []byte {
	return []byte(strings.Join([]string{e.Sender, e.Timestamp.UTC().Format(time.RFC3339Nano), e.Nonce, string(e.Payload)}, "\n"))
}
