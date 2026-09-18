"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";

// Redirects to /login once the shared AuthContext has resolved and found no
// signed-in user. Centralizes the "is this route protected" check - it used
// to be reimplemented per-page as its own /api/check-auth fetch + redirect,
// duplicated across five different pages. AuthContext already fetches this
// once on mount, so a page calling this hook gets the guard for free
// instead of firing a second, redundant network request.
export function useRequireAuth() {
  const { user, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !user) {
      router.push("/login");
    }
  }, [user, loading, router]);

  return { user, loading };
}
