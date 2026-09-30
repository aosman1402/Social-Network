// Inline SVG icons used across the UI in place of emoji. Emoji are drawn
// from whatever color-emoji font the viewer's OS happens to have, so on
// systems without one (many Linux setups, older Windows, some browsers)
// they rendered as blank boxes. SVG is drawn by the browser itself, so
// these look the same everywhere.
//
// Sized in em so an icon scales with the text around it, the same way the
// emoji it replaces did. Each icon has a default color; pass
// color='currentColor' to inherit the text color instead (e.g. on buttons).

const ICONS = {
  globe: {
    color: "#0ea5e9",
    body: (
      <>
        <circle cx='12' cy='12' r='10' />
        <path d='M2 12h20' />
        <path d='M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z' />
      </>
    ),
  },
  users: {
    color: "#6366f1",
    body: (
      <>
        <path d='M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2' />
        <circle cx='9' cy='7' r='4' />
        <path d='M23 21v-2a4 4 0 0 0-3-3.87' />
        <path d='M16 3.13a4 4 0 0 1 0 7.75' />
      </>
    ),
  },
  group: {
    color: "#8b5cf6",
    body: (
      <>
        <circle cx='12' cy='7' r='3' />
        <circle cx='5' cy='9' r='2.2' />
        <circle cx='19' cy='9' r='2.2' />
        <path d='M7 21v-1.5a5 5 0 0 1 10 0V21' />
        <path d='M1.5 20v-1a3.5 3.5 0 0 1 4.5-3.3' />
        <path d='M22.5 20v-1a3.5 3.5 0 0 0-4.5-3.3' />
      </>
    ),
  },
  lock: {
    color: "#f59e0b",
    body: (
      <>
        <rect x='3' y='11' width='18' height='11' rx='2' />
        <path d='M7 11V7a5 5 0 0 1 10 0v4' />
      </>
    ),
  },
  camera: {
    color: "#ec4899",
    body: (
      <>
        <path d='M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z' />
        <circle cx='12' cy='13' r='4' />
      </>
    ),
  },
  calendar: {
    color: "#ef4444",
    body: (
      <>
        <rect x='3' y='4' width='18' height='18' rx='2' />
        <path d='M16 2v4' />
        <path d='M8 2v4' />
        <path d='M3 10h18' />
      </>
    ),
  },
  pin: {
    color: "#ef4444",
    body: (
      <>
        <path d='M12 17v5' />
        <path d='M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z' />
      </>
    ),
  },
  user: {
    color: "#6366f1",
    body: (
      <>
        <path d='M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2' />
        <circle cx='12' cy='7' r='4' />
      </>
    ),
  },
  "user-x": {
    color: "#f43f5e",
    body: (
      <>
        <path d='M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2' />
        <circle cx='9' cy='7' r='4' />
        <path d='M17 8l5 5' />
        <path d='M22 8l-5 5' />
      </>
    ),
  },
  "user-minus": {
    color: "#64748b",
    body: (
      <>
        <path d='M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2' />
        <circle cx='9' cy='7' r='4' />
        <path d='M16 11h6' />
      </>
    ),
  },
  mail: {
    color: "#0ea5e9",
    body: (
      <>
        <rect x='2' y='4' width='20' height='16' rx='2' />
        <path d='M22 6l-10 7L2 6' />
      </>
    ),
  },
  "check-circle": {
    color: "#10b981",
    body: (
      <>
        <path d='M22 11.08V12a10 10 0 1 1-5.93-9.14' />
        <path d='M22 4 12 14.01l-3-3' />
      </>
    ),
  },
  check: {
    color: "#10b981",
    body: <path d='M20 6 9 17l-5-5' />,
  },
  x: {
    color: "#f43f5e",
    body: (
      <>
        <path d='M18 6 6 18' />
        <path d='M6 6l12 12' />
      </>
    ),
  },
  message: {
    color: "#6366f1",
    body: <path d='M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z' />,
  },
  bell: {
    color: "#f59e0b",
    body: (
      <>
        <path d='M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9' />
        <path d='M13.73 21a2 2 0 0 1-3.46 0' />
      </>
    ),
  },
  clock: {
    color: "#f59e0b",
    body: (
      <>
        <circle cx='12' cy='12' r='10' />
        <path d='M12 6v6l4 2' />
      </>
    ),
  },
  info: {
    color: "#0ea5e9",
    body: (
      <>
        <circle cx='12' cy='12' r='10' />
        <path d='M12 16v-4' />
        <path d='M12 8h.01' />
      </>
    ),
  },
  search: {
    color: "#64748b",
    body: (
      <>
        <circle cx='11' cy='11' r='8' />
        <path d='M21 21l-4.35-4.35' />
      </>
    ),
  },
  heart: {
    color: "#f43f5e",
    filled: true,
    body: (
      <path d='M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z' />
    ),
  },
  star: {
    color: "#f59e0b",
    filled: true,
    body: (
      <polygon points='12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2' />
    ),
  },
  cake: {
    color: "#ec4899",
    body: (
      <>
        <path d='M20 21v-8a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8' />
        <path d='M4 16s.5-1 2-1 2.5 2 4 2 2.5-2 4-2 2.5 2 4 2 2-1 2-1' />
        <path d='M2 21h20' />
        <path d='M7 8v3' />
        <path d='M12 8v3' />
        <path d='M17 8v3' />
        <path d='M7 4h.01' />
        <path d='M12 4h.01' />
        <path d='M17 4h.01' />
      </>
    ),
  },
  send: {
    color: "currentColor",
    body: (
      <>
        <path d='M22 2 11 13' />
        <path d='M22 2l-7 20-4-9-9-4z' />
      </>
    ),
  },
  smile: {
    color: "#f59e0b",
    body: (
      <>
        <circle cx='12' cy='12' r='10' />
        <path d='M8 14s1.5 2 4 2 4-2 4-2' />
        <path d='M9 9h.01' />
        <path d='M15 9h.01' />
      </>
    ),
  },
  edit: {
    color: "currentColor",
    body: (
      <>
        <path d='M12 20h9' />
        <path d='M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4z' />
      </>
    ),
  },
  dot: {
    color: "#10b981",
    filled: true,
    body: <circle cx='12' cy='12' r='6' />,
  },
};

export default function Icon({ name, size = "1em", color, className = "", title }) {
  const icon = ICONS[name] || ICONS.bell;
  const stroke = color || icon.color;
  return (
    <svg
      className={`icon ${className}`}
      width={size}
      height={size}
      viewBox='0 0 24 24'
      fill={icon.filled ? stroke : "none"}
      stroke={stroke}
      strokeWidth='2'
      strokeLinecap='round'
      strokeLinejoin='round'
      role={title ? "img" : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : "true"}
      focusable='false'>
      {icon.body}
    </svg>
  );
}
