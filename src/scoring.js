// Sistema di punteggio F1 2025 (nessun punto per il giro veloce).
export const POINTS = {
  race: [25, 18, 15, 12, 10, 8, 6, 4, 2, 1],
  sprint: [8, 7, 6, 5, 4, 3, 2, 1],
};

export function pointsFor(type, position, dnf) {
  if (dnf || !position) return 0;
  return POINTS[type][position - 1] ?? 0;
}

// Ordina per punti; a parità vale il "countback" sui piazzamenti in gara
// (più vittorie, poi più secondi posti, ecc.), come nel regolamento F1.
export function compareStandings(a, b) {
  if (b.points !== a.points) return b.points - a.points;
  const maxPos = Math.max(0, ...a.positions, ...b.positions);
  for (let p = 1; p <= maxPos; p++) {
    const diff = countOf(b.positions, p) - countOf(a.positions, p);
    if (diff !== 0) return diff;
  }
  return a.name.localeCompare(b.name);
}

function countOf(list, value) {
  return list.filter((v) => v === value).length;
}
