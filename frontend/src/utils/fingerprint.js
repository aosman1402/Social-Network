import FingerprintJS from "@fingerprintjs/fingerprintjs";

// FingerprintJS's agent takes a moment to initialize (it collects several
// browser signals) - loaded and cached once per page load rather than
// re-computed on every call.
let agentPromise = null;

/**
 * Get a stable identifier for this browser/device, sent to the backend as
 * the X-Device-Fingerprint header so it can detect a session cookie being
 * used from a different device than the one that logged in.
 * @returns {Promise<string>} The visitor id, or "" if it couldn't be computed
 *   (e.g. server-side rendering, or the library failed to load) - callers
 *   should treat that as "no fingerprint available" rather than an error.
 */
export const getDeviceFingerprint = async () => {
  if (typeof window === "undefined") return "";
  try {
    if (!agentPromise) agentPromise = FingerprintJS.load();
    const agent = await agentPromise;
    const result = await agent.get();
    return result.visitorId;
  } catch (error) {
    console.error("Failed to compute device fingerprint:", error);
    return "";
  }
};
