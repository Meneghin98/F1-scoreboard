package update

import "testing"

func TestSignVerify(t *testing.T) {
	private, public, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("abc  f1-scoreboard-linux-arm64\n")
	sig, err := Sign(private, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(public, data, sig+"\n"); err != nil {
		t.Errorf("firma valida rifiutata: %v", err)
	}
	if err := verify(public, []byte("altro"), sig); err == nil {
		t.Error("firma accettata per dati diversi")
	}
	_, other, _ := GenerateKey()
	if err := verify(other, data, sig); err == nil {
		t.Error("firma accettata con un'altra chiave")
	}
	if err := verify(publicKey, data, sig); err == nil {
		t.Error("firma di una chiave di prova accettata dalla chiave delle release")
	}
}
