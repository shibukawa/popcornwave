// Reports whether anybody is at the keyboard, so a session ends when a person
// leaves rather than when requests stop arriving. Those are different things: a
// page holding a live connection keeps requesting with nobody there, and a
// person reading one page for an hour requests nothing at all.
//
// What is sent is one boolean per tick and, when the clock jumped, how far.
// No key, no coordinate, and no timing pattern leaves this file.

const INTERVAL_MS = 60_000;

// Set by any input and cleared by each report, so the whole of the state is
// "did anything happen since last time".
let active = false;
let lastTick = Date.now();

const mark = () => { active = true; };
for (const type of ["pointerdown", "pointermove", "keydown", "wheel", "scroll", "touchstart"]) {
  // Passive, and it returns immediately once the flag is set, so even
  // pointermove costs nothing measurable.
  addEventListener(type, mark, { passive: true });
}
// A tab becoming visible is interaction; becoming hidden is not, and is left to
// the tick to notice.
addEventListener("visibilitychange", () => { if (!document.hidden) mark(); });

async function tick() {
  const now = Date.now();
  // Nothing reports a machine waking. A gap far larger than the interval is
  // how it is inferred, and it counts as absence rather than as presence.
  const gap = Math.max(0, Math.round((now - lastTick - INTERVAL_MS) / 1000));
  lastTick = now;
  const report = { active, gap };
  active = false;
  try {
    const response = await fetch("/auth/logout/presence", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(report),
      credentials: "same-origin",
    });
    // The server ends the session when it decides nobody is here. Reloading
    // lands on whatever an anonymous visitor sees.
    if (response.status === 401 || response.status === 403) location.reload();
  } catch {
    // A failed report is not a presence claim. The server treats silence as
    // absence, which is the safe direction.
  }
}

setInterval(tick, INTERVAL_MS);
