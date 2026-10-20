package update

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"f1-scoreboard/internal/version"
)

// Install sostituisce exe con la versione scaricata (exe.new) e ne restituisce
// il numero; se non ce n'è nessuna restituisce "" senza fare nulla. La
// versione precedente resta in exe.old, per tornare indietro a mano.
//
// Funziona anche mentre exe è in esecuzione: sia Linux sia Windows permettono
// di rinominare un eseguibile aperto (non di sovrascriverlo), e il programma
// in esecuzione continua a usare il file vecchio, che ora si chiama exe.old.
// La versione nuova parte al prossimo avvio (o con Restart).
func Install(exe string) (string, error) {
	number := Staged(exe)
	if number == "" {
		return "", nil
	}
	// Una versione non più recente (per esempio perché nel frattempo è stato
	// copiato a mano un eseguibile più nuovo) si scarta: sarebbe un ritorno indietro.
	if !version.Newer(number) {
		os.Remove(exe + newSuffix)
		os.Remove(exe + versionSuffix)
		return "", nil
	}
	old := exe + oldSuffix
	// Un .old di un aggiornamento precedente non serve più.
	if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("eliminazione della versione precedente: %w", err)
	}
	if err := os.Rename(exe, old); err != nil {
		return "", fmt.Errorf("spostamento dell'eseguibile in uso: %w", err)
	}
	if err := os.Rename(exe+newSuffix, exe); err != nil {
		// Si rimette a posto quello vecchio, per non restare senza eseguibile.
		if undo := os.Rename(old, exe); undo != nil {
			return "", fmt.Errorf("installazione: %w (e ripristino non riuscito: %v)", err, undo)
		}
		return "", fmt.Errorf("installazione: %w", err)
	}
	os.Remove(exe + versionSuffix)
	return number, nil
}
