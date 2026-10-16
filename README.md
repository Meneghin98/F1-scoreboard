# F1 2025 Scoreboard

Segnapunti web per i tornei tra amici su F1 25.

- **Backend**: Go (solo libreria standard `net/http`), database SQLite con `modernc.org/sqlite` (Go puro, niente CGO)
- **Frontend**: React 19 + TypeScript + Tailwind CSS 4, compilato con Vite e incorporato nel binario con `go:embed`
- **Persistenza**: tutti i dati sono salvati in `data/scoreboard.db`

Il risultato della build è **un unico eseguibile** (~15 MB) che contiene server e frontend.

## Struttura

```
main.go                 avvio, configurazione, spegnimento pulito
cmd/seed/               dati di esempio per lo sviluppo
cmd/record/             registratore della telemetria UDP di F1 25
internal/telemetry/     decodifica dei pacchetti di telemetria (formato 2025)
internal/recording/     formato dei file di registrazione (.f1rec.gz)
internal/api/           handler HTTP delle API REST
internal/scoring/       punteggi F1 2025 e calcolo della classifica
internal/store/         apertura del DB e migrazioni (migrations/NNN_*.sql)
web/src/                frontend React (components/ e lib/ con client API e tipi)
web/dist/               build del frontend, incorporata nel binario (non versionata)
```

## Sviluppo

Servono Go 1.26+ (con `GOTOOLCHAIN=auto`, il default, Go lo scarica da solo) e Node.js 22.12+.

```sh
cd web && npm install    # solo la prima volta
```

Poi, in due terminali:

```sh
go run .                 # API su http://localhost:3000
cd web && npm run dev    # frontend con hot reload su http://localhost:5173 (inoltra /api al server Go)
```

Per avere qualcosa da vedere, `go run ./cmd/seed` inserisce piloti, due tornei e una decina di gare
di esempio (non fa nulla se nel database ci sono già tornei).

Test e controlli:

```sh
go test ./...            # test delle API e del punteggio
cd web && npm run typecheck
```

Variabili d'ambiente opzionali:

| Variabile | Default              | Descrizione                                  |
|-----------|----------------------|----------------------------------------------|
| `PORT`    | `3000`               | Porta HTTP                                   |
| `DB_PATH` | `data/scoreboard.db` | Percorso del database (relativo alla cartella di avvio) |
| `WEB_DIR` | —                    | Se impostata, serve il frontend da questa cartella (es. `web/dist`) invece che dal binario |

## Build e deploy

Prima si compila il frontend, poi il binario Go che lo incorpora. Da Windows (PowerShell):

```powershell
npm --prefix web run build                                                     # frontend → web/dist
go build -o bin/f1-scoreboard.exe .                                            # Windows
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o bin/f1-scoreboard-arm64 .  # Raspberry Pi (OS a 64 bit)
$env:GOOS="linux"; $env:GOARCH="arm";   go build -o bin/f1-scoreboard-arm .    # Raspberry Pi (OS a 32 bit)
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o bin/f1-scoreboard-amd64 .  # PC / thin client Linux
Remove-Item Env:GOOS, Env:GOARCH                                               # torna alla build locale
```

Se si compila Go senza aver prima buildato il frontend, il server parte comunque ma segnala
nel log che il frontend manca. Per il deploy basta copiare l'eseguibile sulla macchina di
destinazione e avviarlo.

## Telemetria di F1 25

Nel gioco: **Impostazioni → Impostazioni telemetria**

| Voce                   | Valore                                                        |
|------------------------|---------------------------------------------------------------|
| Telemetria UDP         | Attiva                                                        |
| Modalità broadcast UDP | Disattivata                                                   |
| Indirizzo IP UDP       | IP del PC con il server (`127.0.0.1` se il gioco gira lì)     |
| Porta UDP              | `20777`                                                       |
| Frequenza di invio UDP | `20 Hz`                                                       |
| Formato UDP            | `2025`                                                        |

Il gioco invia i dati solo quando si è in una sessione (in pista o nel garage), non dai menù.

### Registrare una sessione

```sh
go run ./cmd/record                          # registra in recordings/<data-ora>.f1rec.gz, Ctrl+C per terminare
go run ./cmd/record -info recordings/xxx.f1rec.gz   # riassunto: durata, sessioni, pacchetti, eventi
```

Durante la registrazione vengono segnalati inizio e fine sessione, bandiera a scacchi e arrivo
della classifica finale. Il file viene scaricato su disco ogni secondo, quindi resta leggibile anche
se il programma si interrompe. Le registrazioni (circa 160 KB/s non compressi a 20 Hz) sono escluse
da Git.

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
