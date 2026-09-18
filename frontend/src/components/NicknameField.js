"use client";

import { useRef } from "react";
import { debounce, isValidNicknameFormat } from "@/utils/helpers";
import { apiFetch } from "@/lib/api";

// A nickname input with live availability checking against
// /api/check-nickname. Owns its own debounce/fetch/status lifecycle -
// the caller only gets the current value and status, for building the
// registration payload and gating submission.
export default function NicknameField({
  id = "register-nickname",
  value,
  onValueChange,
  status,
  onStatusChange,
  error,
}) {
  // Tracks the most recently typed value so a slow/late availability
  // response can't overwrite the status for a value the user has since
  // changed (or cleared) - checked before applying the response below.
  const latestValueRef = useRef("");

  const checkAvailability = useRef(
    debounce(async (nickname) => {
      try {
        const { data } = await apiFetch(
          `/check-nickname?nickname=${encodeURIComponent(nickname)}`
        );
        if (latestValueRef.current !== nickname) return; // superseded
        if (!data.valid) onStatusChange("invalid");
        else onStatusChange(data.available ? "available" : "taken");
      } catch (err) {
        if (latestValueRef.current !== nickname) return;
        onStatusChange("error");
      }
    }, 400)
  ).current;

  const handleChange = (e) => {
    const newValue = e.target.value;
    latestValueRef.current = newValue;
    onValueChange(newValue);

    if (!newValue) {
      onStatusChange("idle");
      return;
    }
    if (!isValidNicknameFormat(newValue)) {
      onStatusChange("invalid");
      return;
    }
    onStatusChange("checking");
    checkAvailability(newValue);
  };

  return (
    <div>
      <label htmlFor={id} className='block text-sm font-medium text-gray-700 mb-1'>
        Nickname <span className='text-gray-400 text-xs'>(Optional)</span>
      </label>
      <input
        id={id}
        type='text'
        name='nickname'
        value={value}
        onChange={handleChange}
        maxLength={14}
        className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
        placeholder='3-14 characters, letters and numbers'
      />
      {status === "checking" && (
        <p className='text-gray-500 text-xs mt-1'>○ Checking availability...</p>
      )}
      {status === "available" && (
        <p className='text-green-700 text-xs mt-1'>✓ Nickname is available</p>
      )}
      {status === "taken" && (
        <p className='text-red-500 text-xs mt-1'>✕ Nickname is already taken</p>
      )}
      {status === "invalid" && (
        <p className='text-red-500 text-xs mt-1'>
          ✕ 3-14 characters, letters and numbers only
        </p>
      )}
      {status === "error" && (
        <p className='text-red-500 text-xs mt-1'>
          ✕ Could not check availability - try again
        </p>
      )}
      {/* "invalid"/"taken"/"error" already have their own message above;
          only "checking" needs this one, to explain why submitting while
          the check is still in flight was blocked. */}
      {error && status === "checking" && (
        <p className='text-red-500 text-sm mt-1'>{error}</p>
      )}
    </div>
  );
}
