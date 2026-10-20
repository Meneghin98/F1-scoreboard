# F1 Scoreboard

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
internal/scoring/       punteggi F1 2025, regole dei tornei e calcolo della classifica
internal/store/         apertura del DB e migrazioni (migrations/NNN_*.sql)
internal/backup/        backup del database, da scaricare e automatici
internal/disk/          spazio libero sul disco (un file per sistema operativo)
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
go test ./...            # test delle API, del punteggio, della telemetria e dei backup
cd web && npm run typecheck
cd web && npm run lint   # Biome: errori comuni e dipendenze degli hook di React
```

Il linter del frontend è [Biome](https://biomejs.dev) (configurazione in `web/biome.jsonc`): le
regole consigliate più il controllo delle dipendenze degli hook. Dove una dipendenza manca apposta
c'è un commento `biome-ignore` con il motivo.

Variabili d'ambiente opzionali:

| Variabile          | Default              | Descrizione                                  |
|--------------------|----------------------|----------------------------------------------|
| `PORT`             | `3000`               | Porta HTTP                                   |
| `DB_PATH`          | `data/scoreboard.db` | Percorso del database (relativo alla cartella di avvio) |
| `WEB_DIR`          | —                    | Se impostata, serve il frontend da questa cartella (es. `web/dist`) invece che dal binario |
| `TELEMETRY_ADDR`   | `:20777`             | Indirizzo UDP su cui ricevere la telemetria del gioco |
| `TELEMETRY_DIR`    | `data/sessions`      | Cartella delle sessioni registrate (accanto al database) |
| `TELEMETRY_RECORD` | `true`               | `false` per non registrare le sessioni su file |
| `BACKUP_DIR`       | `data/backups`       | Cartella dei backup automatici (accanto al database) |
| `BACKUP_KEEP`      | `7`                  | Backup automatici da tenere, uno al giorno; `0` per non farne |
| `TZ`               | quello del sistema   | Fuso orario su Linux, es. `Europe/Rome` (vedi sotto) |

Le date delle gare create dalle registrazioni e i nomi dei file usano l'**ora locale del server**.
Sul Raspberry Pi il fuso orario spesso è UTC: va impostato con `sudo timedatectl set-timezone
Europe/Rome`, oppure si avvia il server con `TZ=Europe/Rome`. I dati dei fusi orari sono dentro
l'eseguibile, quindi `TZ` funziona anche sui sistemi che non li hanno.

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

In split-screen la pagina Live mostra anche il **Duello** tra i due giocatori (chi è davanti, di
quanto, confronto dei giri). La **Modalità TV** (`#/tv`, dal pulsante nella pagina Live) mostra
classifica, mappa e direzione gara a schermo intero, con caratteri grandi da leggere dal divano.
Ogni pilota ha un colore (si cambia nella pagina Piloti), lo stesso nei grafici e nella classifica live.

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

### Overlay per lo streaming (OBS)

La pagina `#/overlay` è fatta per chi trasmette le gare con OBS: sopra il gioco mostra la
classifica a torre (le auto della IA con la sigla del pilota, quelle dei giocatori con il nome e
il colore del pilota del segnapunti e P1/P2), circuito e giro, e per qualche secondo ogni nuovo
messaggio della direzione gara. Lo sfondo è trasparente e non ci sono comandi da cliccare; senza
una sessione in corso la pagina è vuota.

In OBS: **Sorgenti → + → Browser**, con

| Campo     | Valore |
|-----------|--------|
| URL       | `http://IP-DEL-SERVER:3000/#/overlay` (`http://localhost:3000/#/overlay` se OBS gira sullo stesso PC del server); l'indirizzo esatto è nella pagina Sistema |
| Larghezza | `1920` |
| Altezza   | `1080` |

Il CSS personalizzato proposto da OBS va bene così. Le dimensioni dell'overlay seguono l'altezza
della sorgente: con 1920×1080 il testo è di circa 19 pixel.

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
   senza risultati) la sessione va lì, altrimenti ne viene creata una nuova. Un torneo si crea,
   si inizia, si termina e se ne cambiano nome e regole di punteggio dalla pagina Tornei;
   terminato, le gare tornano in Nessun torneo, che non si può iniziare né eliminare.

Ogni sessione del weekend finita (con la classifica finale) appartiene sempre a un torneo: dalla
pagina Registrazioni si può spostare in un altro, non togliere. Quando la si sposta, la gara
rimasta vuota in Nessun torneo viene eliminata. Eliminando una gara o un torneo, le sue
registrazioni passano in Nessun torneo; eliminando la gara da Nessun torneo, la registrazione
resta senza torneo. Prove libere (FP) e qualifiche (Q, anche lo Sprint Shootout) compaiono tra
le gare con i piazzamenti, ma non danno punti e non entrano in classifica (le qualifiche sì, se il
torneo dà punti per la pole: vedi [Punteggio](#punteggio)). Le sessioni finite registrate prima
di questa regola sono state spostate in Nessun torneo al primo avvio.

Nella pagina Gare le gare con i risultati presi da una registrazione hanno l'etichetta "Dalla
telemetria" e il link al replay. I risultati si possono correggere a mano: da quel momento
cambiare i piloti della registrazione non li aggiorna più, finché non si ripristinano con
"Ripristina risultati". Le pagine si aggiornano da sole quando i dati cambiano (una gara salvata
dalla telemetria, una modifica fatta da un altro PC).

Il **replay** mostra una registrazione come la pagina Live, con una timeline per spostarsi.
Prepararlo vuol dire rileggere tutta la registrazione: su un Raspberry Pi Zero 2 W, per un'ora di
gara, decine di secondi. Per questo il server prepara il replay delle sessioni del weekend appena
finiscono e lo salva accanto alla registrazione (`<file>.replay`, meno di un decimo della sua
dimensione): aprirlo è immediato. Il file si rifà da solo dopo un aggiornamento del server o se
cambiano i piloti.

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

## Punteggio

Di default vale il regolamento F1 2025:

| Tipo        | Punti dal 1° in giù               |
|-------------|-----------------------------------|
| Gran Premio | 25, 18, 15, 12, 10, 8, 6, 4, 2, 1 |
| Sprint      | 8, 7, 6, 5, 4, 3, 2, 1            |

Il punto per il giro veloce non c'è più dal 2025. A parità di punti vale il countback sui
piazzamenti nei Gran Premi (più vittorie, poi più secondi posti, ecc.).

Ogni torneo ha le sue **regole di punteggio**: si scelgono quando lo si crea e si cambiano da
"Impostazioni" nella pagina Tornei. La classifica si ricalcola da sola, anche per le gare già corse.

- **Punti in base alla posizione d'arrivo** (default), IA compresa come nel gioco: chi arriva 11°
  dietro a dieci auto della IA non prende punti. In alternativa **in base alla posizione tra i
  piloti**: in ogni gara le posizioni si ricalcolano tra i soli piloti del segnapunti (i ritirati
  restano ritirati), poi si applica la tabella. Vittorie, podi e countback del torneo seguono le
  stesse posizioni; nella pagina Gare compaiono accanto a quelle d'arrivo.
- **Punti per la pole** (0 = nessuno): al primo della qualifica, o al primo dei piloti nell'altra
  modalità. Con i punti per la pole le qualifiche entrano nelle colonne della classifica.
- **Punti per il giro veloce** (0 = nessuno): solo nei Gran Premi e solo a chi arriva al traguardo.
  Con le posizioni d'arrivo vanno a chi ha fatto il giro più veloce della gara; tra i piloti, al
  miglior tempo dei piloti (nelle gare inserite a mano, senza tempi, a chi è segnato).

La classifica mostra anche le **pole** (qualifiche vinte) e i **giri veloci** nei Gran Premi, con
un segno sui punti bonus. Il giro veloce arriva dalla registrazione (chi ha fatto il miglior giro
della gara, IA compresa) o si segna a mano nell'editor dei risultati.

## Statistiche

La pagina **Statistiche** raccoglie tutte le gare di tutti i tornei:

- **Carriera**: titoli, punti (somma delle classifiche dei tornei, ognuna con le sue regole),
  vittorie, podi, pole, giri veloci, ritiri, media e miglior piazzamento di ogni pilota. Questi
  ultimi sono quelli d'arrivo, IA compresa, anche nei tornei con i punti tra i piloti;
- **Record dei circuiti**: il giro più veloce dei piloti su ogni circuito, in qualunque sessione
  registrata, con il link al replay;
- **Testa a testa**: quante volte ogni pilota è arrivato davanti a ciascun altro.

Ogni pilota ha un **profilo** (dalla pagina Piloti o cliccando il nome) con i tornei, i migliori
giri per circuito, gli ultimi risultati e i confronti diretti. Miglior giro, griglia, giri, soste
e penalità si leggono dalla classifica finale delle registrazioni; quelle registrate prima di
questa funzione vengono rilette una volta all'avvio del server.

## Backup e spazio su disco

La pagina **Sistema** raccoglie backup, spazio su disco, conservazione delle registrazioni, stato
della telemetria e versione del programma (il commit da cui è stato compilato).

- **Scarica backup** copia il database in quel momento (`VACUUM INTO`, una fotografia coerente
  anche mentre il server lavora) e lo scarica come `f1-scoreboard_AAAA-MM-GG_hh-mm.db`.
- **Backup automatici**: uno al giorno in `data/backups/`, tenendo gli ultimi 7 (vedi `BACKUP_DIR`
  e `BACKUP_KEEP`). Il server lo fa all'avvio se l'ultimo ha più di un giorno, poi controlla ogni
  ora. Dalla pagina Sistema si possono scaricare.
- **Registrazioni**: non sono nei backup (pesano circa 2 MB al minuto). Per conservarle si copia la
  cartella `data/sessions/`. Nella pagina Sistema si può scegliere dopo quanti giorni eliminarne i
  file (di default mai), tenendo eventualmente quelle delle gare dei tornei: sessioni, risultati,
  statistiche e record restano, ma senza file non si possono più rivedere né scaricare.

Per **ripristinare** un backup: fermare il server, sostituire `data/scoreboard.db` con il file del
backup (rinominato `scoreboard.db`), eliminare `scoreboard.db-wal` e `scoreboard.db-shm` se ci
sono, e riavviare.

| Endpoint                     | Descrizione                                                     |
|------------------------------|-----------------------------------------------------------------|
| `GET /api/backup`            | Scarica una copia del database fatta in quel momento            |
| `GET /api/backups`           | Backup automatici, dal più recente                              |
| `GET /api/backups/{nome}`    | Scarica un backup automatico                                    |
| `GET /api/system`            | Versione, spazio di database, registrazioni e backup, disco libero |
| `GET`/`PUT /api/settings`    | Conservazione delle registrazioni (`{"recordings": {"days": 30, "keepTournaments": true}}`) |
