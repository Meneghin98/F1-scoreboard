// Package version contiene il numero di versione del programma, scritto
// nell'eseguibile al momento della build:
//
//	go build -ldflags "-X f1-scoreboard/internal/version.Number=1.0.0" .
//
// Le release pubblicate dalla CI lo hanno sempre; le build fatte a mano senza
// -ldflags (e go run) sono build di sviluppo e non si aggiornano da sole.
package version

import "golang.org/x/mod/semver"

// Number è la versione semver senza la "v" iniziale (es. "1.0.0"), vuota nelle
// build di sviluppo. È una variabile e non una costante perché -X può
// cambiare solo le variabili stringa.
var Number string

// Release dice se l'eseguibile è una release, cioè ha una versione valida.
func Release() bool {
	return semver.IsValid("v" + Number)
}

// Newer dice se la versione other (con o senza "v") è più recente di quella
// dell'eseguibile. Una build di sviluppo non considera mai nulla più recente.
func Newer(other string) bool {
	if !Release() {
		return false
	}
	if other == "" || other[0] != 'v' {
		other = "v" + other
	}
	return semver.IsValid(other) && semver.Compare(other, "v"+Number) > 0
}
