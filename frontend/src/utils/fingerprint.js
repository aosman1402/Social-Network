/**
 * Get a stable identifier for this browser/device, sent to the backend as
 * the X-Device-Fingerprint header so it can detect a session cookie being
 * used from a different device than the one that logged in.
 * Built only from signals the browser already exposes, hashed with the
 * built-in Web Crypto API (SHA-256) - no third-party library.
 * Screen size is deliberately left out: it changes on the same device when
 * the window moves to another monitor, a display is (un)plugged, or
 * DevTools' device toolbar is toggled - and a mismatch logs the user out.
 * @returns {Promise<string>} The hex hash, or "" if it couldn't be computed
 *   (e.g. server-side rendering, or crypto.subtle unavailable outside
 *   HTTPS/localhost) - callers should treat that as "no fingerprint
 *   available" rather than an error.
 */
export const getDeviceFingerprint = async () => {
  if (typeof window === "undefined" || !window.crypto?.subtle) return "";
  try {
    const signals = [
      navigator.userAgent,
      navigator.language,
      navigator.platform,
      navigator.hardwareConcurrency,
      Intl.DateTimeFormat().resolvedOptions().timeZone,
    ].join("|");
    const hash = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(signals));
    return Array.from(new Uint8Array(hash), (b) => b.toString(16).padStart(2, "0")).join("");
  } catch (error) {
    console.error("Failed to compute device fingerprint:", error);
    return "";
  }
};
