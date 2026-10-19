# Lavoro in corso (file temporaneo)

> Da eliminare quando tutto il lavoro qui sotto è finito e unito in `main`.

Questo file serve a riprendere il lavoro da dove è stato lasciato. Branch: `miglioramenti-analisi`.
Le parti 1-3 sono già anche su `main` (e su `origin/main`); il branch aggiunge la parte 4 e questo file.

## Da dove nasce

Il 18-19/10/2026 è stata fatta un'analisi completa dell'app. L'utente ha chiesto di applicare tutte
le correzioni possibili e di implementare quattro nuove funzionalità, con queste precisazioni:

- **niente CI** (GitHub Actions);
- **niente supporto da telefono** (menu mobile, QR code, tooltip pensati per il touch) e **niente
  interventi di sola accessibilità** (ruoli ARIA, WCAG): è un progetto da usare tra amici, su PC;
- delle nuove funzionalità proposte vanno fatte: **1** statistiche e record, **2** regole di
  punteggio per torneo, **3** backup dal browser (più backup automatico), **5** overlay per lo
  streaming. **Non** vanno fatte: notifiche Discord/Telegram e condivisione della classifica come
  immagine.

Il lavoro è diviso in parti: alla fine di ognuna si verifica tutto e si fa un commit (prima di
iniziare la parte successiva il lavoro precedente deve essere su git).

## Fatto

| Commit    | Parte | Contenuto |
|-----------|-------|-----------|
| `902d15d` | —     | Lavoro di un'altra sessione (chiusura delle sessioni finite 10 s dopo l'ultima classifica), solo committato |
| `92cf6bb` | 1     | Telemetria robusta: coda tra lettura UDP ed elaborazione (`listener.go`), buffer UDP da 4 MB, salvataggio dei risultati che riprova dopo un errore (`saveLineup`), recupero delle sessioni non salvate dopo uno spegnimento (`pending_save`, `recoverPending`), diagnostica nella pagina Live (porta occupata, formato UDP sbagliato, IP da impostare nel gioco), stream SSE con gzip, header dei pacchetti senza reflection, stato live non preparato senza browser collegati |
| `478a1be` | 2     | Coerenza: le sessioni riempiono la gara già in calendario (`plannedRace`), eliminando gara/torneo le registrazioni passano in "Nessun torneo" (`races.go`), risultati modificati a mano non riscritti dalla registrazione (`races.manual_results`, `ResyncRace`), versione dei dati nello stato live per il ricaricamento automatico delle pagine (`DataChanged`, `useLoad`), modifica di piloti/tornei/gare, elenco unico dei circuiti (`tracks.go`, `GET /api/tracks`, `races.track_id`), gare in ordine di data, `ReadHeaderTimeout` e limite al body |
| `d7ca853` | 3     | Interfaccia: barra di navigazione, pagina Tornei (azioni sui tornei tolte dall'header), classifica con posizioni, distacco e grafico dei punti (`PointsChart`), colore di ogni pilota (`players.color`), scheda Duello in split-screen (`DuelCard`), modalità TV (`#/tv`), "Caricamento…" |
| `af0a12c` | 4     | Statistiche (funzionalità 1): dati dalla classifica finale (`telemetry_players.best_lap_ms` ecc., riletti all'avvio con `backfillStats`), giro veloce nei risultati (dalla registrazione o a mano), pole e giri veloci in classifica, pagina Statistiche (carriera, record dei circuiti, testa a testa) e profilo del pilota (`#/player/{id}`) |

Tutti i punti della sezione "Logica da correggere" dell'analisi sono fatti. Prossima migrazione
libera: **`011`**.

## Da fare

### Parte 5 — Regole di punteggio per torneo (funzionalità 2)

Scelte già fatte con l'analisi (oggi conta la posizione assoluta, IA compresa: chi arriva 11° con 18
auto della IA prende 0 punti):

- migrazione `011`: `tournaments.points_mode TEXT NOT NULL DEFAULT 'absolute' CHECK (points_mode IN
  ('absolute', 'players'))`, `tournaments.pole_points INTEGER NOT NULL DEFAULT 0`,
  `tournaments.fastest_lap_points INTEGER NOT NULL DEFAULT 0`. I valori di default danno il
  comportamento di oggi;
- `players`: in ogni gara le posizioni si ricalcolano tra i soli piloti del segnapunti con un
  risultato (i DNF restano DNF), poi si applica la tabella dei punti;
- bonus pole: al primo della qualifica (assoluto, o tra i piloti in modalità `players`), assegnato
  nella gara di qualifica, che con `pole_points > 0` entra nelle colonne della classifica
  (`scoring.Counts` deve dipendere dalle regole);
- bonus giro veloce: in modalità `absolute` a chi ha `fastest_lap`; in `players` a chi ha il
  `best_lap_ms` più basso tra i risultati della gara (se non ci sono tempi, come nelle gare
  inserite a mano, vale `fastest_lap`); solo a chi non è DNF; solo Gran Premi;
- codice: `scoring.Standings` e `PointsFor` ricevono le regole; le API dei tornei leggono/scrivono
  i tre campi (`tournamentColumns`, `createTournament`, `updateTournament`); i punti di
  `listRaces`, `getStandings`, il capoclassifica di `listTournaments` e `statsData.standings`
  (`internal/api/stats.go`) usano le regole del torneo;
- interfaccia: impostazioni nel dialog del nuovo torneo e nella pagina Tornei (il modulo "Rinomina"
  può diventare "Impostazioni"); in classifica un segno per i bonus (pole, giro veloce);
  aggiornare la sezione "Punteggio" del README;
- test: modalità `players` e bonus in `scoring_test.go`, impostazioni nelle API.

### Parte 6 — Backup e spazio su disco (funzionalità 3 e punti dell'analisi)

- `GET /api/backup`: `VACUUM INTO` in un file temporaneo nella cartella dei dati, inviato come
  download (`f1-scoreboard_AAAA-MM-GG_hh-mm.db`) e poi eliminato;
- backup automatico giornaliero in `data/backups` (cartella accanto al DB), tenendo gli ultimi N
  (variabile `BACKUP_KEEP`, default 7): all'avvio se l'ultimo ha più di 24 ore, poi controllo ogni
  ora; elenco e download (`GET /api/backups`, `GET /api/backups/{nome}`, accettando solo nomi del
  formato dei backup);
- `GET /api/system`: spazio occupato dalle registrazioni (e quante sono), dimensione del DB, spazio
  libero sul disco (`golang.org/x/sys`, già tra le dipendenze indirette: `unix.Statfs` su Linux,
  `windows.GetDiskFreeSpaceEx` su Windows, con build tag), versione da
  `runtime/debug.ReadBuildInfo` (`vcs.revision`, `vcs.time`, `vcs.modified`);
- conservazione delle registrazioni: tabella `settings` (chiave/valore) con
  `recordings_retention_days` (0 = per sempre) e "tieni le gare dei tornei"; il controllo di ogni
  ora elimina solo i **file** (righe, risultati e statistiche restano), la pagina Registrazioni
  già gestisce "File non disponibile";
- pagina **Sistema** (`#/system`, nella barra): backup, elenco dei backup automatici, spazio su
  disco, impostazioni di conservazione, versione, stato della telemetria;
- fuso orario: `import _ "time/tzdata"` in `main.go` e nota nel README (sul Raspberry impostare
  `TZ=Europe/Rome`): le date delle gare usano l'ora locale del server;
- aggiornare la sezione "Backup" del README.

### Parte 7 — Replay più veloce

Preparare il replay di un'ora di gara su un Raspberry Pi Zero 2 W può richiedere decine di secondi.

- a fine sessione (sessioni del weekend con classifica) preparare il replay in background e
  salvarlo accanto alla registrazione (`<file>.replay`, gob di fotogrammi, pedali, giri, eventi,
  analisi e pianta), con un'intestazione che identifica l'eseguibile (`os.Executable()`, data di
  modifica e dimensione): dopo ogni nuova build il file non vale più e si rifà;
- `loadReplay` usa il file se è valido, altrimenti prepara il replay e lo salva;
- eliminare il file in `removeSession` e quando cambiano i piloti (`SetSessionPlayers` chiama già
  `forgetReplay`).

### Parte 8 — Overlay per lo streaming (funzionalità 5) e linter

- pagina `#/overlay` a schermo intero come `#/tv` (aggiungerla a `STANDALONE_VIEWS`), con sfondo
  trasparente per la sorgente "Browser" di OBS: torre della classifica compatta (colori dei
  piloti, P1/P2), circuito e giro, ultimo messaggio della direzione gara; nessun comando
  cliccabile; spiegare nel README come aggiungerla in OBS (URL `http://IP:3000/#/overlay`);
- linter per il frontend: **Biome** (solo lint, regole consigliate più `useExhaustiveDependencies`),
  perché il progetto usa TypeScript 7 e il parser di ESLint potrebbe non supportarlo; script
  `npm run lint`; dove le dipendenze degli hook sono omesse apposta (`useLoad`, `useTrackMap`…)
  usare `biome-ignore` con il motivo;
- PIN facoltativo per le modifiche (`ADMIN_PIN`, spento di default): nell'analisi era indicato come
  necessario solo prima di esporre l'app su internet. Chiedere all'utente se lo vuole.

### Alla fine

- riepilogo all'utente: cosa è stato fatto, cosa è stato saltato e perché (CI, telefono,
  accessibilità, Discord/Telegram, immagine della classifica), dipendenze aggiunte;
- nel riepilogo correggere una stima dell'analisi: sulla schermata dei risultati il gioco ripete
  la classifica finale ogni 5 secondi, quindi il rischio di perderla era più basso di quanto
  stimato (la coda resta utile);
- eliminare questo file.

## Come verificare

- `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix web run typecheck`;
- server di prova senza toccare quello dell'utente (che può girare su `:3000` e UDP `20777`):
  `npm --prefix web run build`, poi compilare in una cartella temporanea con un nome unico
  (`go build -o <tmp>/f1test-server.exe .`) e avviarlo con `PORT=3101 DB_PATH=<tmp>/data/scoreboard.db
  TELEMETRY_ADDR=:20901`. Per fermarlo `taskkill //F //IM f1test-server.exe`: **mai** terminare per
  nome processi generici come `server.exe` o `msedge.exe`;
- dati di esempio: `DB_PATH=<tmp>/data/scoreboard.db go run ./cmd/seed` (il "Campionato 2025" ha una
  gara a Monza in calendario senza risultati);
- telemetria: `go run ./cmd/record -replay testdata/recordings/2026-10-16_13-31-38.f1rec.gz -to
  127.0.0.1:20901 -speed 10` (la gara è l'ultima sessione; HAMILTON, auto 19, è il giocatore);
- split-screen: la registrazione ha un solo giocatore; per simularne due basta rimandarla
  riscrivendo il byte 28 dell'header di ogni pacchetto (`SecondaryPlayerCarIndex`, per esempio 5)
  con un piccolo script Node (`zlib.gunzipSync`, formato in `internal/recording`);
- screenshot: `puppeteer-core` installato in una cartella temporanea fuori dal progetto, che usa
  Edge (`C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe`); prima dello screenshot
  aspettare un testo della pagina, perché lo stream SSE tiene la rete sempre occupata (niente
  `networkidle` né `--virtual-time-budget`).

## Convenzioni

- Tutto in italiano: interfaccia, commenti, messaggi di commit. L'utente conosce Go a livello
  junior: commentare le scelte non ovvie, senza esagerare.
- Commit con titolo all'imperativo e corpo che spiega il perché, chiusi da
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Migrazioni: solo file nuovi numerati in sequenza, mai modificare quelli esistenti.
- Le API che modificano dati fanno già ricaricare le pagine aperte (`handle` chiama
  `live.Service.DataChanged`); il servizio live lo fa da solo quando salva una gara.
- Si possono aggiungere librerie quando servono davvero, indicandole nel riepilogo; restano i
  vincoli del progetto: binario unico, niente CGO.
