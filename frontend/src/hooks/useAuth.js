"use client";

import { useContext, useCallback, useMemo } from "react";
import { AuthContext } from "@/context/AuthContext";

/**
 * Custom hook for authentication with additional utilities
 * @returns {Object} Auth context value and helper functions
 */
export const useAuth = () => {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }

  const { user, loading, login, register, logout } = context;

  // Check if user has a specific role
  const hasRole = useCallback(
    (role) => {
      if (!user) return false;
      return user.role === role || user.roles?.includes(role);
    },
    [user],
  );

  // Check if user is admin
  const isAdmin = useMemo(() => {
    return hasRole("admin");
  }, [hasRole]);

  // Get user's full name
  const fullName = useMemo(() => {
    if (!user) return "";
    return `${user.first_name || ""} ${user.last_name || ""}`.trim();
  }, [user]);

  // Get user's display name (nickname or full name)
  const displayName = useMemo(() => {
    if (!user) return "";
    return user.nickname || fullName || user.email || "User";
  }, [user, fullName]);

  // Get user's initials for avatar
  const initials = useMemo(() => {
    if (!user) return "";
    const firstName = user.first_name || "";
    const lastName = user.last_name || "";
    return `${firstName.charAt(0)}${lastName.charAt(0)}`.toUpperCase();
  }, [user]);

  // Check if user is authenticated and has completed profile
  const hasCompletedProfile = useMemo(() => {
    if (!user) return false;
    return !!(user.first_name && user.last_name && user.email);
  }, [user]);

  return {
    // Core auth state
    user,
    loading,
    isAuthenticated: !!user,

    // Core auth methods
    login,
    register,
    logout,

    // User info helpers
    fullName,
    displayName,
    initials,

    // Permission helpers
    hasRole,
    isAdmin,

    // Profile status
    hasCompletedProfile,

    // Convenience methods
    isLoggedIn: !!user,
    isGuest: !user,
  };
};

export default useAuth;
