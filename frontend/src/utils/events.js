const pad = (n) => String(n).padStart(2, "0");

/**
 * Convert an <input type="datetime-local"> value ("2026-10-01T18:30"),
 * which is in the browser's local time, to RFC3339 with that timezone's
 * offset ("2026-10-01T18:30:00+01:00") - the format CreateEvent expects.
 * Returns null for an empty or invalid value.
 */
export const localInputToRFC3339 = (value) => {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;

  const offsetMinutes = -date.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const abs = Math.abs(offsetMinutes);
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}:00` +
    `${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
  );
};

/** The current local time as a datetime-local value, for the input's min. */
export const nowAsLocalInput = (now = new Date()) =>
  `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}` +
  `T${pad(now.getHours())}:${pad(now.getMinutes())}`;

/** Split an event's responses into going / not going lists. */
export const splitResponses = (responses = []) => ({
  going: responses.filter((r) => r.response === "going"),
  notGoing: responses.filter((r) => r.response === "not_going"),
});
