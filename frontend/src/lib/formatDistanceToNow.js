// Plain-JS replacement for date-fns' formatDistanceToNow(date, { addSuffix }).
// Only needs to handle past dates (created_at timestamps), formatted as
// "5 minutes ago", "2 hours ago", "3 days ago", etc.

const UNITS = [
  { unit: "year", seconds: 31536000 },
  { unit: "month", seconds: 2592000 },
  { unit: "day", seconds: 86400 },
  { unit: "hour", seconds: 3600 },
  { unit: "minute", seconds: 60 },
];

export function formatDistanceToNow(date, { addSuffix = false } = {}) {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);

  if (seconds < 45) {
    return addSuffix ? "less than a minute ago" : "less than a minute";
  }

  for (const { unit, seconds: unitSeconds } of UNITS) {
    const count = Math.floor(seconds / unitSeconds);
    if (count >= 1) {
      const label = `${count} ${unit}${count > 1 ? "s" : ""}`;
      return addSuffix ? `${label} ago` : label;
    }
  }

  return addSuffix ? "less than a minute ago" : "less than a minute";
}
