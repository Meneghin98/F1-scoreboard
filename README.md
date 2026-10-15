# F1 2025 Scoreboard

Segnapunti web per i tornei tra amici su F1 25.

- **Backend**: Go (solo libreria standard `net/http`), database SQLite con `modernc.org/sqlite` (Go puro, niente CGO)
- **Frontend**: HTML/CSS/JavaScript senza framework, incorporato nel binario con `go:embed`
- **Persistenza**: tutti i dati sono salvati in `data/scoreboard.db`

Il risultato della build è **un unico eseguibile** (~15 MB) che contiene server e frontend.

## Struttura

```
main.go                 avvio, configurazione, spegnimento pulito
internal/api/           handler HTTP delle API REST
internal/scoring/       punteggi F1 2025 e calcolo della classifica
internal/store/         apertura del DB e migrazioni (migrations/NNN_*.sql)
web/public/             frontend
```

## Sviluppo

Serve Go 1.26 o superiore (con `GOTOOLCHAIN=auto`, il default, Go lo scarica da solo).

```sh
go run .                 # http://localhost:3000
go test ./...            # test delle API e del punteggio
```

Per vedere subito le modifiche al frontend senza ricompilare, servilo dal disco:

```powershell
$env:WEB_DIR = "web/public"; go run .
```

Variabili d'ambiente opzionali:

| Variabile | Default              | Descrizione                                  |
|-----------|----------------------|----------------------------------------------|
| `PORT`    | `3000`               | Porta HTTP                                   |
| `DB_PATH` | `data/scoreboard.db` | Percorso del database (relativo alla cartella di avvio) |
| `WEB_DIR` | —                    | Se impostata, serve il frontend da questa cartella invece che dal binario |

## Build e deploy

Da Windows (PowerShell) si compila per qualsiasi destinazione:

```powershell
go build -o bin/f1-scoreboard.exe .                                          # Windows
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o bin/f1-scoreboard-arm64 .  # Raspberry Pi (OS a 64 bit)
$env:GOOS="linux"; $env:GOARCH="arm";   go build -o bin/f1-scoreboard-arm .    # Raspberry Pi (OS a 32 bit)
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o bin/f1-scoreboard-amd64 .  # PC / thin client Linux
Remove-Item Env:GOOS, Env:GOARCH                                               # torna alla build locale
```

Per il deploy basta copiare l'eseguibile sulla macchina di destinazione e avviarlo.

## Database

Lo schema è gestito da migrazioni SQL in `internal/store/migrations/`, applicate all'avvio.
La versione corrente è salvata in `PRAGMA user_version`. Per modificare lo schema si aggiunge un
nuovo file con il numero successivo (es. `002_telemetria.sql`): i file già esistenti non vanno
mai modificati. I database creati dalla vecchia versione Node.js vengono adottati senza modifiche.

## Punteggio (regolamento F1 2025)

| Tipo        | Punti dal 1° in giù               |
|-------------|-----------------------------------|
| Gran Premio | 25, 18, 15, 12, 10, 8, 6, 4, 2, 1 |
| Sprint      | 8, 7, 6, 5, 4, 3, 2, 1            |

Il punto per il giro veloce non c'è più dal 2025. A parità di punti vale il countback sui
piazzamenti nei Gran Premi (più vittorie, poi più secondi posti, ecc.).

## Backup

Per fare un backup basta copiare la cartella `data/` a server fermo.
