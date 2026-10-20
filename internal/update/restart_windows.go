package update

import (
	"os"
	"os/exec"
)

// Restart avvia exe con gli stessi argomenti, variabili d'ambiente e console,
// poi chiude questo processo. Windows non ha un exec che sostituisce il
// processo: il server nuovo è un processo diverso, che continua a scrivere
// nella stessa finestra e si ferma con Ctrl+C come prima. Se riesce non ritorna.
func Restart(exe string) error {
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
