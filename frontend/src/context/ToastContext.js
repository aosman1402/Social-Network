"use client";

import { useState, useCallback, useEffect } from "react";

// Minimal replacement for react-hot-toast. `toast` is a plain module-level
// object so it can be imported and called from anywhere (`toast.success(...)`,
// `toast.error(...)`) exactly like before - no hook/context needed at the
// call site. ToastProvider renders the actual popups and subscribes to
// whatever gets emitted.
let listeners = [];
let idCounter = 0;

function emit(type, message) {
  const id = ++idCounter;
  listeners.forEach((fn) => fn({ id, type, message }));
  return id;
}

export const toast = {
  success: (message) => emit("success", message),
  error: (message) => emit("error", message),
};

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([]);

  const remove = useCallback((id) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  useEffect(() => {
    const handler = (t) => {
      setToasts((prev) => [...prev, t]);
      setTimeout(() => remove(t.id), 3000);
    };
    listeners.push(handler);
    return () => {
      listeners = listeners.filter((fn) => fn !== handler);
    };
  }, [remove]);

  return (
    <>
      {children}
      <div className='toast-container'>
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`toast toast-${t.type}`}
            onClick={() => remove(t.id)}>
            {t.message}
          </div>
        ))}
      </div>
    </>
  );
}
