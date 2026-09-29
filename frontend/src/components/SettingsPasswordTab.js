"use client";

import { useState } from "react";
import { isPasswordValid } from "@/utils/helpers";
import PasswordRequirements from "@/components/PasswordRequirements";
import { apiFetch } from "@/lib/api";
import { useAuth } from "@/context/AuthContext";

// The "Password" tab of the Settings page. On success, every session
// (including this one) is invalidated server-side, so the caller is sent
// back to /login.
export default function SettingsPasswordTab({
  isLoading,
  setIsLoading,
  setMessage,
  onChanged, // () => void - called after a successful password change
}) {
  const { user } = useAuth();
  const [password, setPassword] = useState({
    current: "",
    new: "",
    confirm: "",
  });

  const handlePasswordChange = (e) => {
    const { name, value } = e.target;
    setPassword({ ...password, [name]: value });
  };

  const handlePasswordSubmit = async (e) => {
    e.preventDefault();
    if (password.new !== password.confirm) {
      setMessage("Passwords do not match");
      return;
    }
    if (!isPasswordValid(password.new)) {
      setMessage("New password does not meet all requirements below");
      return;
    }

    setIsLoading(true);
    setMessage("");

    try {
      const { ok, data } = await apiFetch("/user/password", {
        method: "PUT",
        json: {
          current_password: password.current,
          new_password: password.new,
        },
      });

      if (!ok) {
        setMessage(data.error || "Failed to change password");
        return;
      }

      setPassword({ current: "", new: "", confirm: "" });
      onChanged();
    } catch (error) {
      console.error("Error changing password:", error);
      setMessage("Failed to change password");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <form onSubmit={handlePasswordSubmit} className='space-y-4'>
      <h2 className='text-lg font-semibold'>Change Password</h2>
      <p className='text-sm text-gray-500'>12 to 15 characters required</p>

      {/* Hidden username field so browsers and password managers know which
          account's saved password to update (and Chrome stops warning that
          the form has none). Never submitted - only the passwords are sent. */}
      <input
        type='text'
        name='username'
        autoComplete='username'
        value={user?.email || ""}
        readOnly
        hidden
      />

      <div>
        <label
          htmlFor='settings-current-password'
          className='block text-sm font-medium text-gray-700 mb-1'>
          Current Password
        </label>
        <input
          id='settings-current-password'
          autoComplete='current-password'
          type='password'
          name='current'
          value={password.current}
          onChange={handlePasswordChange}
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
          required
        />
      </div>

      <div>
        <label
          htmlFor='settings-new-password'
          className='block text-sm font-medium text-gray-700 mb-1'>
          New Password
        </label>
        <input
          id='settings-new-password'
          autoComplete='new-password'
          type='password'
          name='new'
          value={password.new}
          onChange={handlePasswordChange}
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
          required
        />
        <PasswordRequirements password={password.new} />
      </div>

      <div>
        <label
          htmlFor='settings-confirm-new-password'
          className='block text-sm font-medium text-gray-700 mb-1'>
          Confirm New Password
        </label>
        <input
          id='settings-confirm-new-password'
          autoComplete='new-password'
          type='password'
          name='confirm'
          value={password.confirm}
          onChange={handlePasswordChange}
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
          required
        />
      </div>

      <button
        type='submit'
        disabled={isLoading}
        className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
        {isLoading ? "Changing..." : "Change Password"}
      </button>
    </form>
  );
}
