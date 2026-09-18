"use client";

import { createContext, useContext, useState, useEffect } from "react";
import { getDeviceFingerprint } from "@/utils/fingerprint";
import { apiFetch } from "@/lib/api";

const AuthContext = createContext();

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  const checkAuth = async () => {
    try {
      const { data } = await apiFetch("/check-auth", {
        headers: { "X-Device-Fingerprint": await getDeviceFingerprint() },
      });
      if (data.authenticated) {
        setUser(data);
        return data;
      } else {
        setUser(null);
        return null;
      }
    } catch (error) {
      console.error("Auth check error:", error);
      setUser(null);
      return null;
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    checkAuth();
  }, []);

  const login = async (email, password) => {
    try {
      const { ok, data } = await apiFetch("/login", {
        method: "POST",
        headers: { "X-Device-Fingerprint": await getDeviceFingerprint() },
        json: { email, password },
      });

      if (!ok) {
        throw new Error(data.message || data.error || "Login failed");
      }

      const userData = await checkAuth();
      return { success: true, user: userData };
    } catch (error) {
      console.error("Login error:", error);
      return { success: false, error: error.message };
    }
  };

  const logout = async () => {
    try {
      await apiFetch("/logout", { method: "POST" });
      localStorage.clear();
      sessionStorage.clear();
      setUser(null);
      return { success: true };
    } catch (error) {
      console.error("Logout error:", error);
      return { success: false, error: error.message };
    }
  };

  const value = {
    user,
    loading,
    setUser,
    login,
    logout,
    checkAuth,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
