"use client";

import { useState, useEffect } from "react";
import { toast } from "@/context/ToastContext";
import { getAvatarSrc } from "@/utils/helpers";
import { apiFetch } from "@/lib/api";

// The "Profile" tab of the Settings page: avatar, name, bio, and the
// public/private toggle. Loads the signed-in user's real profile on mount
// (the shared auth user from /api/check-auth doesn't carry about_me).
export default function SettingsProfileTab({
  user,
  setUser,
  isLoading,
  setIsLoading,
  setMessage,
}) {
  const [profile, setProfile] = useState({
    firstName: "",
    lastName: "",
    email: "",
    bio: "",
    isPublic: true,
  });
  const [profileLoaded, setProfileLoaded] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  // The last-saved value, so a confirmation only fires when isPublic is
  // actually being changed by this submit, not on every unrelated save.
  const [savedIsPublic, setSavedIsPublic] = useState(true);

  useEffect(() => {
    if (!user) return;

    const loadProfile = async () => {
      try {
        const { ok, data } = await apiFetch(`/user/profile?id=${user.id}`);
        if (ok) {
          setProfile({
            firstName: data.first_name || "",
            lastName: data.last_name || "",
            email: data.email || "",
            bio: data.about_me || "",
            isPublic: !!data.is_public,
          });
          setSavedIsPublic(!!data.is_public);
        } else {
          toast.error("Failed to load profile");
        }
      } catch (error) {
        console.error("Error loading profile:", error);
        toast.error("Failed to load profile");
      } finally {
        setProfileLoaded(true);
      }
    };

    loadProfile();
  }, [user]);

  const handleAvatarChange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;

    const validTypes = ["image/jpeg", "image/png", "image/gif"];
    if (!validTypes.includes(file.type)) {
      toast.error("Please upload a valid image (JPEG, PNG, or GIF)");
      e.target.value = "";
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      toast.error("Image size must be less than 5MB");
      e.target.value = "";
      return;
    }

    setUploadingAvatar(true);
    try {
      const formData = new FormData();
      formData.append("avatar", file);

      const { ok, data } = await apiFetch("/user/avatar", {
        method: "POST",
        body: formData,
      });

      if (!ok) {
        toast.error(data.error || "Failed to update avatar");
        return;
      }

      setUser((prev) => (prev ? { ...prev, avatar_path: data.avatar_path } : prev));
      toast.success("Profile picture updated!");
    } catch (error) {
      console.error("Error uploading avatar:", error);
      toast.error("Failed to update avatar");
    } finally {
      setUploadingAvatar(false);
      e.target.value = "";
    }
  };

  const handleProfileChange = (e) => {
    const { name, value, type, checked } = e.target;
    setProfile({
      ...profile,
      [name]: type === "checkbox" ? checked : value,
    });
  };

  const handleProfileSubmit = async (e) => {
    e.preventDefault();

    if (profile.isPublic !== savedIsPublic) {
      const confirmMessage = profile.isPublic
        ? "Switching your profile to public will automatically accept all of your pending follow requests and notify those users. Continue?"
        : "Switching your profile to private means new followers will need your approval, and only accepted followers will be able to see your posts and profile. Continue?";
      if (!confirm(confirmMessage)) {
        setProfile((prev) => ({ ...prev, isPublic: savedIsPublic }));
        return;
      }
    }

    setIsLoading(true);
    setMessage("");

    try {
      const { ok, data } = await apiFetch("/user/update", {
        method: "PUT",
        json: {
          first_name: profile.firstName,
          last_name: profile.lastName,
          about_me: profile.bio,
          is_public: profile.isPublic,
        },
      });

      if (!ok) {
        setMessage(data.error || "Failed to update profile");
        return;
      }

      setMessage("Profile updated successfully!");
      setSavedIsPublic(profile.isPublic);
      // Keep the shared auth user (navbar, profile page, etc.) in sync.
      setUser((prev) =>
        prev
          ? {
              ...prev,
              first_name: profile.firstName,
              last_name: profile.lastName,
              is_public: profile.isPublic,
            }
          : prev,
      );
    } catch (error) {
      console.error("Error updating profile:", error);
      setMessage("Failed to update profile");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <form onSubmit={handleProfileSubmit} className='space-y-4'>
      <h2 className='text-lg font-semibold'>Profile Information</h2>

      <div className='flex items-center gap-4'>
        {user?.avatar_path ? (
          <img
            src={getAvatarSrc(user.avatar_path)}
            alt='Profile picture'
            className='w-16 h-16 rounded-full object-cover border'
          />
        ) : (
          <div className='w-16 h-16 rounded-full bg-blue-500 flex items-center justify-center text-white text-xl font-bold'>
            {user?.first_name?.[0] || "U"}
          </div>
        )}
        <label className='cursor-pointer px-3 py-2 border border-gray-300 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition'>
          {uploadingAvatar ? "Uploading..." : "Change Photo"}
          <input
            id='settings-avatar-file'
            name='avatar'
            type='file'
            accept='image/jpeg,image/png,image/gif'
            onChange={handleAvatarChange}
            className='hidden'
            disabled={uploadingAvatar}
          />
        </label>
      </div>

      <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
        <div>
          <label
            htmlFor='settings-first-name'
            className='block text-sm font-medium text-gray-700 mb-1'>
            First Name
          </label>
          <input
            id='settings-first-name'
            type='text'
            name='firstName'
            value={profile.firstName}
            onChange={handleProfileChange}
            className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
          />
        </div>
        <div>
          <label
            htmlFor='settings-last-name'
            className='block text-sm font-medium text-gray-700 mb-1'>
            Last Name
          </label>
          <input
            id='settings-last-name'
            type='text'
            name='lastName'
            value={profile.lastName}
            onChange={handleProfileChange}
            className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
          />
        </div>
      </div>

      <div>
        <label
          htmlFor='settings-email'
          className='block text-sm font-medium text-gray-700 mb-1'>
          Email
        </label>
        <input
          id='settings-email'
          type='email'
          name='email'
          value={profile.email}
          disabled
          readOnly
          title="Email can't be changed here"
          className='w-full px-3 py-2 border border-gray-300 rounded-lg bg-gray-50 text-gray-500 cursor-not-allowed'
        />
      </div>

      <div>
        <label
          htmlFor='settings-bio'
          className='block text-sm font-medium text-gray-700 mb-1'>
          About Me
        </label>
        <textarea
          id='settings-bio'
          name='bio'
          value={profile.bio}
          onChange={handleProfileChange}
          rows='3'
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
        />
      </div>

      <div className='flex items-center space-x-2'>
        <input
          id='settings-is-public'
          type='checkbox'
          name='isPublic'
          checked={profile.isPublic}
          onChange={handleProfileChange}
          className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
        />
        <label htmlFor='settings-is-public' className='text-sm text-gray-700'>
          Make my profile public
        </label>
      </div>

      <button
        type='submit'
        disabled={isLoading || !profileLoaded}
        className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
        {isLoading ? "Saving..." : !profileLoaded ? "Loading..." : "Save Profile"}
      </button>
    </form>
  );
}
