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
internal/telemetry/     parser dei pacchetti di telemetria (formati 2025 e 2026)
internal/recording/     formato dei file di registrazione (.f1rec.gz)
internal/live/          ascolto UDP, stato live della sessione, registrazione automatica
internal/api/           handler HTTP delle API REST
internal/scoring/       punteggi F1 2025 e calcolo della classifica
internal/store/         apertura del DB e migrazioni (migrations/NNN_*.sql)
web/src/                frontend React (pages/, components/, components/ui/ e lib/)
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

| Variabile          | Default              | Descrizione                                  |
|--------------------|----------------------|----------------------------------------------|
| `PORT`             | `3000`               | Porta HTTP                                   |
| `DB_PATH`          | `data/scoreboard.db` | Percorso del database (relativo alla cartella di avvio) |
| `WEB_DIR`          | —                    | Se impostata, serve il frontend da questa cartella (es. `web/dist`) invece che dal binario |
| `TELEMETRY_ADDR`   | `:20777`             | Indirizzo UDP su cui ricevere la telemetria del gioco |
| `TELEMETRY_DIR`    | `data/sessions`      | Cartella delle sessioni registrate (accanto al database) |
| `TELEMETRY_RECORD` | `true`               | `false` per non registrare le sessioni su file |

## Build e deploy

Prima si compila il frontend, poi il binario Go che lo incorpora. Da Windows (PowerShell):

```powershell
npm --prefix web run build                                                     # frontend → web/dist (con le versioni .gz)
go build -o bin/f1-scoreboard.exe .                                            # Windows
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o bin/f1-scoreboard-arm64 .  # Raspberry Pi (OS a 64 bit)
$env:GOOS="linux"; $env:GOARCH="arm";   go build -o bin/f1-scoreboard-arm .    # Raspberry Pi (OS a 32 bit)
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o bin/f1-scoreboard-amd64 .  # PC / thin client Linux
Remove-Item Env:GOOS, Env:GOARCH                                               # torna alla build locale
```

Il server invia ai browser la versione `.gz` dei file JS e CSS e fa tenere in cache per sempre
i file in `assets/`, che cambiano nome a ogni build.

Se si compila Go senza aver prima buildato il frontend, il server parte comunque ma segnala
nel log che il frontend manca. Per il deploy basta copiare l'eseguibile sulla macchina di
destinazione e avviarlo.

## Telemetria di F1 25

### Ascolto sempre attivo

Il server ascolta la telemetria appena parte, senza configurazioni: appena il gioco entra in
una sessione questa viene rilevata, mostrata nell'header ("Monza · Gara · Giro 2/5") e
nella pagina Live con la classifica in tempo reale, e
**registrata automaticamente** in `data/sessions/`, una registrazione per sessione (circa
2 MB al minuto). Se la porta UDP è occupata (per esempio da `cmd/record`) il server parte
comunque e riprova ogni 10 secondi.

Quando non c'è una sessione, la pagina Live mostra cosa impostare nel gioco (con gli indirizzi IP
del server) e i problemi di ricezione: porta UDP occupata o formato UDP non supportato. I
pacchetti ricevuti aspettano in una coda prima di essere elaborati, così un rallentamento (per
esempio una scrittura lenta sul disco) non ne fa perdere.

| Endpoint                                  | Descrizione                                              |
|-------------------------------------------|----------------------------------------------------------|
| `GET /api/live`                           | Stato della sessione in corso: circuito, giro, piloti, distacchi, gomme |
| `GET /api/live/stream`                    | Lo stesso stato in tempo reale (Server-Sent Events, max 4 aggiornamenti/s) |
| `PUT /api/live/lineup`                    | Sceglie chi guida le auto dei controller nella gara in corso, o conferma |
| `GET /api/telemetry/sessions`             | Sessioni registrate                                      |
| `GET /api/telemetry/sessions/{id}/download` | Scarica la registrazione di una sessione (`.f1rec.gz`) |

### Chi guida? (split-screen) e salvataggio delle sessioni

Il gioco dice quale auto guida ciascun controller, ma non chi lo tiene in mano. In ogni
sessione del weekend (prove libere, qualifiche, gara) la pagina Live mostra **Chi guida?** con
i piloti già proposti (quelli che avevano lo stesso controller nella sessione precedente):

1. **Prima della partenza** si possono cambiare; senza interventi la scelta si conferma da sola
   allo spegnimento del semaforo (fuori dalla gara, al secondo giro) o dopo 5 minuti. Cambiare un
   pilota ferma il conto alla rovescia.
2. **A fine sessione** compare la classifica dei giocatori da confermare; senza interventi viene
   salvata dopo 5 minuti (e comunque quando finisce la sessione). Se il salvataggio non riesce,
   la pagina Live mostra l'errore e il server riprova da solo dopo 30 secondi; se si spegne di
   colpo prima di salvare, la sessione viene salvata al riavvio.
3. La sessione diventa una gara del **torneo in corso** o, se nessun torneo è in corso, di
   **Nessun torneo**. Se nel torneo c'è già la gara in calendario (stesso circuito e tipo, ancora
   senza risultati) la sessione va lì, altrimenti ne viene creata una nuova. Un torneo si inizia
   e si termina dai pulsanti nell'header; terminato, le gare tornano in Nessun torneo, che non si
   può iniziare né eliminare.

Ogni sessione del weekend finita (con la classifica finale) appartiene sempre a un torneo: dalla
pagina Registrazioni si può spostare in un altro, non togliere. Quando la si sposta, la gara
rimasta vuota in Nessun torneo viene eliminata. Eliminando una gara o un torneo, le sue
registrazioni passano in Nessun torneo; eliminando la gara da Nessun torneo, la registrazione
resta senza torneo. Prove libere (FP) e qualifiche (Q, anche lo Sprint Shootout) compaiono tra
le gare con i piazzamenti, ma non danno punti e non entrano in classifica. Le sessioni finite
registrate prima di questa regola sono state spostate in Nessun torneo al primo avvio.

Nella pagina Gare le gare con i risultati presi da una registrazione hanno l'etichetta "Dalla
telemetria" e il link al replay. I risultati si possono correggere a mano: da quel momento
cambiare i piloti della registrazione non li aggiorna più, finché non si ripristinano con
"Ripristina risultati". Le pagine si aggiornano da sole quando i dati cambiano (una gara salvata
dalla telemetria, una modifica fatta da un altro PC).

Se il gioco gira su un altro dispositivo (console o altro PC), su Windows serve una regola del
firewall per la porta UDP 20777 (vedi sotto per l'IP da impostare nel gioco).

Nel gioco: **Impostazioni → Impostazioni telemetria**

| Voce                   | Valore                                                        |
|------------------------|---------------------------------------------------------------|
| Telemetria UDP         | Attiva                                                        |
| Modalità broadcast UDP | Disattivata                                                   |
| Indirizzo IP UDP       | IP del PC con il server (`127.0.0.1` se il gioco gira lì)     |
| Porta UDP              | `20777`                                                       |
| Frequenza di invio UDP | `20 Hz`                                                       |
| Formato UDP            | `2025`, oppure `2026` con il DLC 2026 Season Pack             |

Il gioco invia i dati solo quando si è in una sessione (in pista o nel garage), non dai menù.
Con il DLC 2026 Season Pack il formato 2026 aggiunge le 24 auto e il pacchetto Car Telemetry 2:
controlla quale formato è impostato nel gioco, il registratore lo mostra a ogni nuova sessione.

### Registrare una sessione

```sh
go run ./cmd/record                          # registra in recordings/<data-ora>.f1rec.gz, Ctrl+C per terminare
go run ./cmd/record -info recordings/xxx.f1rec.gz   # riassunto: sessioni, circuito, classifica finale, eventi
go run ./cmd/record -replay recordings/xxx.f1rec.gz -speed 10   # la rimanda al server come se fosse il gioco
```

Durante la registrazione vengono segnalati inizio e fine sessione, bandiera a scacchi e arrivo
della classifica finale. Il file viene scaricato su disco ogni secondo, quindi resta leggibile anche
se il programma si interrompe. Le registrazioni (circa 160 KB/s non compressi a 20 Hz) finiscono in
`recordings/`, esclusa da Git: quelle da conservare vanno copiate in `testdata/recordings/`,
salvata con Git LFS.

### Parser

`internal/telemetry` decodifica tutti i pacchetti di F1 25 nei formati 2025 e 2026
(`telemetry.Parse`). È verificato in due modi:

- **formato 2025**: `go test ./internal/telemetry` decodifica tutte le registrazioni in
  `testdata/recordings` controllando che ogni pacchetto venga letto byte per byte e che i valori
  siano plausibili (serve `git lfs pull` per scaricarle);
- **formato 2026**: in mancanza di registrazioni, ogni pacchetto viene confrontato con la dimensione
  ufficiale della specifica EA. Le forze G del 2026 sono esposte grezze perché EA non ne documenta
  la scala.

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
