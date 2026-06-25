# Catalog sync: schedule and visibility

Beekeeper keeps its threat-intel catalogs fresh with a background sync. This page
explains the schedule, where to see what the sync is doing, and how it alerts you
when a sync hit quarantines an installed package.

## Schedule (heartbeat vs interval)

The unprivileged background job (`beekeeper catalogs daemon install`) wakes
`beekeeper catalogs sync` on an HOURLY heartbeat. The hourly wake-up is not the
fetch rate. Inside each run an interval gate checks whether the configured cadence
has elapsed since the last success; if not, the run prints `sync skipped: not due`
and exits.

- Default cadence: `2h` (configurable `2h` to `24h` via `catalog_sync.interval`,
  or the dashboard catalogs panel `i` key).
- The heartbeat stays hourly by design so the OS schedule never has to be rewritten
  when you change the interval. With the 2h default you fetch at most every two
  hours; the hourly wake-ups in between are no-ops.

## Silent, logged background runs

The OS scheduler runs `catalogs sync --background`. In that mode Beekeeper:

- Hides its console window on Windows (so the hourly heartbeat does not flash a
  blank terminal). macOS launchd and Linux systemd already run detached, so there
  is nothing to hide there. On Windows 11 the installer uses `conhost --headless`
  when available for a true zero-flash run, otherwise the binary hides its own
  console.
- Tees all output to `<state>/logs/sync.log` (size-rotated, single `sync.log.1`
  backup) on every platform. This is the durable record of what each run did. On
  macOS launchd would otherwise discard the output entirely.

The `--background` flag is passed automatically by all three installers (schtasks,
launchd, systemd). You do not run it by hand; a manual `beekeeper catalogs sync`
(or `--force`) still prints to your terminal as before.

## Seeing the last result: `catalogs status`

```
beekeeper catalogs status
```

reports the persisted summary of the most recent run:

- result (`synced`, `unchanged`, `skipped`, `disabled`, or `error`) and entry count
- scan hits, and how many were quarantined / pending / would-quarantine (dry-run)
- the next-due time
- whether the background daemon is registered
- the `sync.log` path

The same summary drives a `last sync:` line in the dashboard catalogs panel.

## On a sync hit: quarantine and alert

When a sync makes the catalog index fresher, the first-responder cross-references
your installed packages (the Pollen/Bumblebee inventory) against the updated index.
A match that meets the corroboration threshold (`auto_quarantine.threshold`,
default 2) is reversibly moved to quarantine when auto-quarantine is enabled, and:

- a best-effort desktop notification fires (Windows toast / macOS Notification
  Center / Linux notify-send) so you are alerted even with the dashboard closed.
  A headless Linux box with no display sends nothing. Dry-run mode audits the hit
  but moves nothing and sends no "quarantined" notification.
- if the dashboard is open, the catalog-quarantine card is raised, with human-gated
  `[r]`estore / `[p]`urge / `[a]`cknowledge. Restore and purge run in the
  quarantine panel (admin-gated, with confirmation). Purge is never automatic.

Quarantine is always a reversible move; the destructive purge stays human-gated.
`catalogs status` is read-only: it reports, it does not itself fetch or block.
