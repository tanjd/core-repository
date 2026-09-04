/**
 * Thin wrapper around window.open for external navigation (e.g. the
 * Telegram bot deep link) in a new tab. Exists so callers can be tested
 * without fighting jsdom's Location/Window objects, which lock down nearly
 * every property as non-configurable.
 *
 * Deliberately omits the usual "noopener,noreferrer" window features:
 * per spec, window.open returns null when either is set, since there's no
 * safe reference to hand back — which defeats the caller's whole reason for
 * opening the tab (setting its location once an async value is ready).
 * That protection guards against an untrusted destination using
 * window.opener to redirect the page that opened it; every caller here
 * hardcodes the destination URL itself, so there's nothing for that
 * protection to guard against.
 */
export function openNewTab(url: string): Window | null {
  return window.open(url, "_blank");
}
