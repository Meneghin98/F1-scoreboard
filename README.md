# F1 2025 Scoreboard

Segnapunti web per i tornei tra amici su F1 25.

- **Backend**: Node.js + Express, database SQLite (modulo integrato `node:sqlite`)
- **Frontend**: HTML/CSS/JavaScript senza framework
- **Persistenza**: tutti i dati sono salvati in `data/scoreboard.db`

## Avvio

Serve Node.js 22.13 o superiore.

```sh
npm install
npm start        # http://localhost:3000
npm run dev      # riavvio automatico quando modifichi i file
npm test         # smoke test delle API su un DB temporaneo
```

Variabili d'ambiente opzionali: `PORT` (default `3000`) e `DB_PATH` (default `data/scoreboard.db`).

## Punteggio (regolamento F1 2025)

| Tipo        | Punti dal 1° in giù               |
|-------------|-----------------------------------|
| Gran Premio | 25, 18, 15, 12, 10, 8, 6, 4, 2, 1 |
| Sprint      | 8, 7, 6, 5, 4, 3, 2, 1            |

Il punto per il giro veloce non c'è più dal 2025. A parità di punti vale il countback sui
piazzamenti nei Gran Premi (più vittorie, poi più secondi posti, ecc.).

## Backup

Per fare un backup basta copiare la cartella `data/` a server fermo.
