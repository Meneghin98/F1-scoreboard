//go:build unix

package update

import (
	"os"
	"syscall"
)

// Restart sostituisce il processo in esecuzione con exe, con gli stessi
// argomenti e variabili d'ambiente. Con exec il processo resta lo stesso
// (stesso PID): per systemd o per chi l'ha avviato da terminale il server non
// si è mai fermato. Se riesce non ritorna.
func Restart(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
