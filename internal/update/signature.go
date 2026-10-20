// Package update aggiorna il programma con le release pubblicate su GitHub.
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

// publicKey verifica la firma del file dei checksum di ogni release. La chiave
// privata corrispondente è solo nel secret UPDATE_SIGNING_KEY del repository
// GitHub: chi riuscisse a caricare un file nelle release senza averla non
// potrebbe far installare un eseguibile diverso.
const publicKey = "nqxmZ08k1pbxsSOjXy09X6ZqAlkIbZc6RWdk6zk+fZs="

// Nomi dei file di ogni release, oltre agli eseguibili.
const (
	ChecksumsName = "checksums.txt"
	SignatureName = "checksums.txt.sig"
)

// Sign firma data con la chiave privata (il seme da 32 byte in base64) e
// restituisce la firma in base64, come va scritta in checksums.txt.sig.
func Sign(privateKey string, data []byte) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKey))
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("chiave privata non valida: servono 32 byte in base64")
	}
	sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// verify controlla che signature (in base64) sia la firma di data fatta con la
// chiave privata delle release.
func verify(key string, data []byte, signature string) error {
	pub, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("chiave pubblica delle release non valida")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil || !ed25519.Verify(pub, data, sig) {
		return errors.New("la firma dei checksum non è valida")
	}
	return nil
}

// GenerateKey crea una nuova coppia di chiavi, entrambe in base64: la privata
// (il seme) va nel secret di GitHub, la pubblica in publicKey.
func GenerateKey() (private, public string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv.Seed()), base64.StdEncoding.EncodeToString(pub), nil
}
