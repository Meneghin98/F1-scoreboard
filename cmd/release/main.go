// Comando release: strumenti per pubblicare le versioni.
//
//	go run ./cmd/release keygen             crea una nuova coppia di chiavi per firmare le release
//	go run ./cmd/release sign checksums.txt firma il file con la chiave in UPDATE_SIGNING_KEY
//
// La firma viene scritta accanto al file, con estensione .sig. Lo usa il
// workflow .github/workflows/release.yml; a mano serve solo keygen.
package main

import (
	"fmt"
	"log"
	"os"

	"f1-scoreboard/internal/update"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("uso: release keygen | release sign <file>")
	}
	switch os.Args[1] {
	case "keygen":
		private, public, err := update.GenerateKey()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Chiave privata (secret UPDATE_SIGNING_KEY su GitHub, non va nel repository):")
		fmt.Println(private)
		fmt.Println()
		fmt.Println("Chiave pubblica (publicKey in internal/update/signature.go):")
		fmt.Println(public)
	case "sign":
		if len(os.Args) != 3 {
			log.Fatal("uso: release sign <file>")
		}
		key := os.Getenv("UPDATE_SIGNING_KEY")
		if key == "" {
			log.Fatal("manca la chiave privata nella variabile UPDATE_SIGNING_KEY")
		}
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		sig, err := update.Sign(key, data)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(os.Args[2]+".sig", []byte(sig+"\n"), 0o644); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("comando sconosciuto %q", os.Args[1])
	}
}
