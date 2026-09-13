"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { toast } from "@/context/ToastContext";
import { getAvatarSrc, isPasswordValid } from "@/utils/helpers";
import PasswordRequirements from "@/components/PasswordRequirements";

export default function SettingsPage() {
  const router = useRouter();
  const { user, setUser, loading: authLoading } = useAuth();
  const [activeTab, setActiveTab] = useState("profile");
  const [isLoading, setIsLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [uploadingAvatar, setUploadingAvatar] = useState(false);

  // Profile settings - starts empty and is populated from the real
  // logged-in user's profile once it loads (see the effect below), instead
  // of shipping hardcoded placeholder data.
  const [profile, setProfile] = useState({
    firstName: "",
    lastName: "",
    email: "",
    bio: "",
    isPublic: true,
  });
  const [profileLoaded, setProfileLoaded] = useState(false);

  // Redirect signed-out visitors, and load this user's real profile
  // (first/last name, about_me, is_public) from the same endpoint the
  // profile page uses - /api/check-auth doesn't carry about_me.
  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      router.push("/login");
      return;
    }

    const loadProfile = async () => {
      try {
        const response = await fetch(
          `http://localhost:8081/api/user/profile?id=${user.id}`,
          { credentials: "include" },
        );
        if (response.ok) {
          const data = await response.json();
          setProfile({
            firstName: data.first_name || "",
            lastName: data.last_name || "",
            email: data.email || "",
            bio: data.about_me || "",
            isPublic: !!data.is_public,
          });
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
  }, [user, authLoading, router]);

  // Password settings
  const [password, setPassword] = useState({
    current: "",
    new: "",
    confirm: "",
  });

  // Notification settings
  const [notifications, setNotifications] = useState({
    email: true,
    push: true,
    follow: true,
    messages: true,
    groups: true,
  });

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

      const response = await fetch("http://localhost:8081/api/user/avatar", {
        method: "POST",
        credentials: "include",
        body: formData,
      });

      const data = await response.json();

      if (!response.ok) {
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

  const handlePasswordChange = (e) => {
    const { name, value } = e.target;
    setPassword({ ...password, [name]: value });
  };

  const handleNotificationChange = (e) => {
    const { name, checked } = e.target;
    setNotifications({ ...notifications, [name]: checked });
  };

  const handleProfileSubmit = async (e) => {
    e.preventDefault();
    setIsLoading(true);
    setMessage("");

    try {
      const response = await fetch("http://localhost:8081/api/user/update", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          first_name: profile.firstName,
          last_name: profile.lastName,
          about_me: profile.bio,
          is_public: profile.isPublic,
        }),
      });

      const data = await response.json();

      if (!response.ok) {
        setMessage(data.error || "Failed to update profile");
        return;
      }

      setMessage("Profile updated successfully!");
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
      const response = await fetch("http://localhost:8081/api/user/password", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          current_password: password.current,
          new_password: password.new,
        }),
      });

      const data = await response.json();

      if (!response.ok) {
        setMessage(data.error || "Failed to change password");
        return;
      }

      setPassword({ current: "", new: "", confirm: "" });
      // Changing the password invalidates every session, including this
      // one - the user has to log in again with the new password.
      router.push("/login");
    } catch (error) {
      console.error("Error changing password:", error);
      setMessage("Failed to change password");
    } finally {
      setIsLoading(false);
    }
  };

  const handleNotificationSubmit = (e) => {
    e.preventDefault();
    setIsLoading(true);
    setMessage("");

    // Simulate API call
    setTimeout(() => {
      setMessage("Notification settings updated!");
      setIsLoading(false);
    }, 1000);
  };

  return (
    <div className='max-w-4xl mx-auto py-4 px-4'>
      <div className='bg-white rounded-lg shadow'>
        {/* Header */}
        <div className='p-6 border-b'>
          <h1 className='text-2xl font-bold text-gray-800'>Settings</h1>
          <p className='text-gray-500 text-sm'>
            Manage your account preferences
          </p>
        </div>

        <div className='flex flex-col md:flex-row'>
          {/* Sidebar */}
          <div className='w-full md:w-48 border-r p-4 space-y-2'>
            <button
              onClick={() => setActiveTab("profile")}
              className={`w-full text-left px-3 py-2 rounded-lg transition ${
                activeTab === "profile"
                  ? "bg-blue-50 text-blue-600"
                  : "hover:bg-gray-50 text-gray-700"
              }`}>
              Profile
            </button>
            <button
              onClick={() => setActiveTab("password")}
              className={`w-full text-left px-3 py-2 rounded-lg transition ${
                activeTab === "password"
                  ? "bg-blue-50 text-blue-600"
                  : "hover:bg-gray-50 text-gray-700"
              }`}>
              Password
            </button>
            <button
              onClick={() => setActiveTab("notifications")}
              className={`w-full text-left px-3 py-2 rounded-lg transition ${
                activeTab === "notifications"
                  ? "bg-blue-50 text-blue-600"
                  : "hover:bg-gray-50 text-gray-700"
              }`}>
              Notifications
            </button>
          </div>

          {/* Content */}
          <div className='flex-1 p-6'>
            {message && (
              <div
                className={`p-3 rounded-lg mb-4 ${
                  message.includes("successfully")
                    ? "bg-green-50 text-green-700"
                    : "bg-red-50 text-red-700"
                }`}>
                {message}
              </div>
            )}

            {/* Profile Tab */}
            {activeTab === "profile" && (
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
                  <label
                    htmlFor='settings-is-public'
                    className='text-sm text-gray-700'>
                    Make my profile public
                  </label>
                </div>

                <button
                  type='submit'
                  disabled={isLoading || !profileLoaded}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                  {isLoading
                    ? "Saving..."
                    : !profileLoaded
                      ? "Loading..."
                      : "Save Profile"}
                </button>
              </form>
            )}

            {/* Password Tab */}
            {activeTab === "password" && (
              <form onSubmit={handlePasswordSubmit} className='space-y-4'>
                <h2 className='text-lg font-semibold'>Change Password</h2>
                <p className='text-sm text-gray-500'>
                  12 to 15 characters required
                </p>

                <div>
                  <label
                    htmlFor='settings-current-password'
                    className='block text-sm font-medium text-gray-700 mb-1'>
                    Current Password
                  </label>
                  <input
                    id='settings-current-password'
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
            )}

            {/* Notifications Tab */}
            {activeTab === "notifications" && (
              <form onSubmit={handleNotificationSubmit} className='space-y-4'>
                <h2 className='text-lg font-semibold'>
                  Notification Preferences
                </h2>
                <p className='text-sm text-gray-500'>
                  Choose what notifications you want to receive
                </p>

                <div className='space-y-3'>
                  <div className='flex items-center justify-between'>
                    <label
                      htmlFor='settings-notif-email'
                      className='text-sm text-gray-700'>
                      Email Notifications
                    </label>
                    <input
                      id='settings-notif-email'
                      type='checkbox'
                      name='email'
                      checked={notifications.email}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label
                      htmlFor='settings-notif-push'
                      className='text-sm text-gray-700'>
                      Push Notifications
                    </label>
                    <input
                      id='settings-notif-push'
                      type='checkbox'
                      name='push'
                      checked={notifications.push}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label
                      htmlFor='settings-notif-follow'
                      className='text-sm text-gray-700'>
                      Follow Notifications
                    </label>
                    <input
                      id='settings-notif-follow'
                      type='checkbox'
                      name='follow'
                      checked={notifications.follow}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label
                      htmlFor='settings-notif-messages'
                      className='text-sm text-gray-700'>
                      Message Notifications
                    </label>
                    <input
                      id='settings-notif-messages'
                      type='checkbox'
                      name='messages'
                      checked={notifications.messages}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label
                      htmlFor='settings-notif-groups'
                      className='text-sm text-gray-700'>
                      Group Notifications
                    </label>
                    <input
                      id='settings-notif-groups'
                      type='checkbox'
                      name='groups'
                      checked={notifications.groups}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                </div>

                <button
                  type='submit'
                  disabled={isLoading}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                  {isLoading ? "Saving..." : "Save Settings"}
                </button>
              </form>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
