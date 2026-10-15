const state = {
  tournaments: [],
  players: [],
  tournamentId: Number(localStorage.getItem('tournamentId')) || null,
  tab: 'standings',
  editingRaceId: null,
};

const $ = (sel) => document.querySelector(sel);

// --- Utility --------------------------------------------------------------

async function api(path, options = {}) {
  const res = await fetch(`/api${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json' },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Errore ${res.status}`);
  return data;
}

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, (c) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));
}

let toastTimer;
function toast(message) {
  const el = $('#toast');
  el.textContent = message;
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { el.hidden = true; }, 3500);
}

async function run(action) {
  try {
    await action();
  } catch (err) {
    toast(err.message);
  }
}

function formatDate(iso) {
  return iso ? new Date(`${iso}T00:00:00`).toLocaleDateString('it-IT') : '';
}

// Etichette brevi per le colonne: R1, R2… per i GP, S1, S2… per le sprint.
function raceLabels(races) {
  let r = 0;
  let s = 0;
  return new Map(races.map((race) => [race.id, race.type === 'sprint' ? `S${++s}` : `R${++r}`]));
}

// --- Tornei ---------------------------------------------------------------

async function loadTournaments() {
  state.tournaments = await api('/tournaments');
  if (!state.tournaments.some((t) => t.id === state.tournamentId)) {
    state.tournamentId = state.tournaments[0]?.id ?? null;
  }
  localStorage.setItem('tournamentId', state.tournamentId ?? '');

  $('#tournament-select').innerHTML = state.tournaments
    .map((t) => `<option value="${t.id}" ${t.id === state.tournamentId ? 'selected' : ''}>${esc(t.name)}</option>`)
    .join('') || '<option value="">Nessun torneo</option>';
  $('#delete-tournament').disabled = !state.tournamentId;
}

$('#tournament-select').addEventListener('change', (e) => run(async () => {
  state.tournamentId = Number(e.target.value);
  localStorage.setItem('tournamentId', state.tournamentId);
  state.editingRaceId = null;
  await render();
}));

$('#new-tournament').addEventListener('click', () => run(async () => {
  const name = prompt('Nome del torneo:', `Torneo F1 2025 #${state.tournaments.length + 1}`);
  if (!name?.trim()) return;
  const t = await api('/tournaments', { method: 'POST', body: { name } });
  state.tournamentId = t.id;
  await loadTournaments();
  await render();
}));

$('#delete-tournament').addEventListener('click', () => run(async () => {
  const t = state.tournaments.find((x) => x.id === state.tournamentId);
  if (!t || !confirm(`Eliminare il torneo "${t.name}" con tutte le sue gare?`)) return;
  await api(`/tournaments/${t.id}`, { method: 'DELETE' });
  await loadTournaments();
  await render();
}));

// --- Tab ------------------------------------------------------------------

document.querySelectorAll('.tab').forEach((btn) => {
  btn.addEventListener('click', () => run(async () => {
    state.tab = btn.dataset.tab;
    document.querySelectorAll('.tab').forEach((b) => b.classList.toggle('active', b === btn));
    await render();
  }));
});

async function render() {
  const needsTournament = state.tab !== 'players';
  const missing = needsTournament && !state.tournamentId;
  $('#no-tournament').hidden = !missing;
  for (const tab of ['standings', 'races', 'players']) {
    $(`#tab-${tab}`).hidden = state.tab !== tab || missing;
  }
  if (missing) return;

  if (state.tab === 'standings') await renderStandings();
  if (state.tab === 'races') await renderRaces();
  if (state.tab === 'players') await renderPlayers();
}

// --- Classifica -----------------------------------------------------------

async function renderStandings() {
  const { races, standings } = await api(`/tournaments/${state.tournamentId}/standings`);
  if (!standings.length) {
    $('#standings').innerHTML = '<p class="empty">Nessun risultato ancora. Aggiungi una gara e inserisci i risultati.</p>';
    return;
  }

  const labels = raceLabels(races);
  const raceCells = (s) => races.map((race) => {
    const r = s.perRace[race.id];
    if (!r) return '<td class="num muted">–</td>';
    if (r.dnf) return '<td class="num dnf" title="Ritirato">DNF</td>';
    return `<td class="num" title="${r.position}° posto">${r.points}</td>`;
  }).join('');

  $('#standings').innerHTML = `
    <div class="card table-wrap">
      <table>
        <thead>
          <tr>
            <th>Pos</th><th>Pilota</th>
            ${races.map((race) => `<th class="num" title="${esc(race.track)}">${labels.get(race.id)}</th>`).join('')}
            <th class="num">Vitt.</th><th class="num">Podi</th><th class="num">DNF</th><th class="num">Punti</th>
          </tr>
        </thead>
        <tbody>
          ${standings.map((s) => `
            <tr>
              <td class="rank rank-${s.rank}">${s.rank}</td>
              <td>${esc(s.name)} ${s.team ? `<span class="team">${esc(s.team)}</span>` : ''}</td>
              ${raceCells(s)}
              <td class="num">${s.wins}</td>
              <td class="num">${s.podiums}</td>
              <td class="num">${s.dnf}</td>
              <td class="num points">${s.points}</td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

// --- Gare -----------------------------------------------------------------

async function renderRaces() {
  const [races, players] = await Promise.all([
    api(`/tournaments/${state.tournamentId}/races`),
    api('/players'),
  ]);
  state.players = players;
  const playerName = (id) => players.find((p) => p.id === id)?.name ?? '?';
  const labels = raceLabels(races);

  if (!races.length) {
    $('#races').innerHTML = '<p class="empty">Nessuna gara in questo torneo.</p>';
    return;
  }

  $('#races').innerHTML = races.map((race) => {
    const editing = state.editingRaceId === race.id;
    const resultsTable = race.results.length ? `
      <table>
        <thead><tr><th>Pos</th><th>Pilota</th><th class="num">Punti</th></tr></thead>
        <tbody>
          ${race.results.map((r) => `
            <tr>
              <td class="${r.dnf ? 'dnf' : `rank rank-${r.position}`}">${r.dnf ? 'DNF' : r.position}</td>
              <td>${esc(playerName(r.player_id))}</td>
              <td class="num">${r.points}</td>
            </tr>`).join('')}
        </tbody>
      </table>` : '<p class="muted">Nessun risultato inserito.</p>';

    return `
      <div class="card" data-race="${race.id}">
        <div class="race-header">
          <h3>
            <span class="muted">${labels.get(race.id)}</span> ${esc(race.track)}
            ${race.type === 'sprint' ? '<span class="badge badge-sprint">Sprint</span>' : ''}
            ${race.date ? `<span class="muted">· ${formatDate(race.date)}</span>` : ''}
          </h3>
          <div class="race-actions">
            ${editing ? '' : `<button class="btn btn-small" data-action="edit">${race.results.length ? 'Modifica risultati' : 'Inserisci risultati'}</button>`}
            <button class="btn btn-ghost btn-small" data-action="delete">Elimina</button>
          </div>
        </div>
        ${editing ? resultsEditor(race, players) : `<div class="table-wrap">${resultsTable}</div>`}
      </div>`;
  }).join('');
}

function resultsEditor(race, players) {
  if (!players.length) {
    return '<p class="muted">Aggiungi prima dei piloti nella scheda "Piloti".</p>';
  }
  const current = new Map(race.results.map((r) => [r.player_id, r]));
  return `
    <form class="results-editor">
      <p class="muted">Inserisci la posizione d'arrivo; lascia vuoto chi non ha partecipato.</p>
      <table>
        <thead><tr><th>Pilota</th><th>Posizione</th><th>DNF</th></tr></thead>
        <tbody>
          ${players.map((p) => {
            const r = current.get(p.id);
            return `
              <tr data-player="${p.id}">
                <td>${esc(p.name)} ${p.team ? `<span class="team">${esc(p.team)}</span>` : ''}</td>
                <td><input type="number" min="1" step="1" name="position" value="${r && !r.dnf ? r.position : ''}"></td>
                <td><input type="checkbox" name="dnf" ${r?.dnf ? 'checked' : ''}></td>
              </tr>`;
          }).join('')}
        </tbody>
      </table>
      <div class="editor-actions">
        <button type="button" class="btn btn-ghost" data-action="cancel">Annulla</button>
        <button class="btn">Salva risultati</button>
      </div>
    </form>`;
}

$('#races').addEventListener('click', (e) => {
  const action = e.target.dataset.action;
  const raceId = Number(e.target.closest('[data-race]')?.dataset.race);
  if (!action || !raceId) return;

  run(async () => {
    if (action === 'edit') state.editingRaceId = raceId;
    if (action === 'cancel') state.editingRaceId = null;
    if (action === 'delete') {
      if (!confirm('Eliminare questa gara e i relativi risultati?')) return;
      await api(`/races/${raceId}`, { method: 'DELETE' });
      if (state.editingRaceId === raceId) state.editingRaceId = null;
    }
    await renderRaces();
  });
});

// Spuntare DNF svuota la posizione e viceversa.
$('#races').addEventListener('input', (e) => {
  const row = e.target.closest('tr[data-player]');
  if (!row) return;
  if (e.target.name === 'dnf' && e.target.checked) row.querySelector('[name=position]').value = '';
  if (e.target.name === 'position' && e.target.value) row.querySelector('[name=dnf]').checked = false;
});

$('#races').addEventListener('submit', (e) => {
  e.preventDefault();
  const raceId = Number(e.target.closest('[data-race]').dataset.race);
  const results = [...e.target.querySelectorAll('tr[data-player]')]
    .map((row) => ({
      player_id: Number(row.dataset.player),
      position: row.querySelector('[name=position]').value,
      dnf: row.querySelector('[name=dnf]').checked,
    }))
    .filter((r) => r.position !== '' || r.dnf)
    .map((r) => ({ ...r, position: r.dnf ? null : Number(r.position) }));

  run(async () => {
    await api(`/races/${raceId}/results`, { method: 'PUT', body: { results } });
    state.editingRaceId = null;
    toast('Risultati salvati');
    await renderRaces();
  });
});

$('#race-form').addEventListener('submit', (e) => {
  e.preventDefault();
  const form = e.target;
  run(async () => {
    const race = await api(`/tournaments/${state.tournamentId}/races`, {
      method: 'POST',
      body: { track: form.track.value, type: form.type.value, date: form.date.value },
    });
    form.reset();
    state.editingRaceId = race.id;
    await renderRaces();
  });
});

// --- Piloti ---------------------------------------------------------------

async function renderPlayers() {
  state.players = await api('/players');
  $('#players').innerHTML = state.players.length
    ? state.players.map((p) => `
        <div class="card player-row" data-player="${p.id}">
          <span>${esc(p.name)} ${p.team ? `<span class="team">${esc(p.team)}</span>` : ''}</span>
          <button class="btn btn-ghost btn-small" data-action="delete">Elimina</button>
        </div>`).join('')
    : '<p class="empty">Nessun pilota registrato.</p>';
}

$('#players').addEventListener('click', (e) => {
  if (e.target.dataset.action !== 'delete') return;
  const id = Number(e.target.closest('[data-player]').dataset.player);
  const player = state.players.find((p) => p.id === id);
  if (!confirm(`Eliminare il pilota "${player?.name}"?`)) return;
  run(async () => {
    await api(`/players/${id}`, { method: 'DELETE' });
    await renderPlayers();
  });
});

$('#player-form').addEventListener('submit', (e) => {
  e.preventDefault();
  const form = e.target;
  run(async () => {
    await api('/players', { method: 'POST', body: { name: form.name.value, team: form.team.value } });
    form.reset();
    form.name.focus();
    await renderPlayers();
  });
});

// --- Avvio ----------------------------------------------------------------

run(async () => {
  await loadTournaments();
  await render();
});
