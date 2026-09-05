"use client";

import { createContext, useContext, useState, useEffect } from "react";

const AuthContext = createContext();

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  const checkAuth = async () => {
    try {
      const response = await fetch("http://localhost:8081/api/check-auth", {
        credentials: "include",
      });
      const data = await response.json();
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
      const response = await fetch("http://localhost:8081/api/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ email, password }),
      });

      const data = await response.json();

      if (!response.ok) {
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
      await fetch("http://localhost:8081/api/logout", {
        method: "POST",
        credentials: "include",
      });
      localStorage.removeItem("user");
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
