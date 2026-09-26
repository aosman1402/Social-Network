// Round "delete" icon button used on posts and events. The click handler
// (and its "Are you sure…?" confirm) stays with the caller.
export default function DeleteButton({ onClick, label = "Delete", className = "" }) {
  return (
    <button
      type='button'
      onClick={onClick}
      className={`delete-btn ${className}`}
      title={label}
      aria-label={label}>
      <svg
        viewBox='0 0 24 24'
        width='16'
        height='16'
        fill='none'
        stroke='currentColor'
        strokeWidth='2'
        strokeLinecap='round'
        strokeLinejoin='round'
        aria-hidden='true'>
        <path d='M3 6h18' />
        <path d='M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2' />
        <path d='M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6' />
        <path d='M10 11v6M14 11v6' />
      </svg>
    </button>
  );
}
