"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import LoadingSpinner from "@/components/LoadingSpinner";

export default function ProfileRedirect() {
  const router = useRouter();
  const { user, loading } = useAuth();

  useEffect(() => {
    if (loading) return;
    router.replace(user ? `/profile/${user.id}` : "/login");
  }, [user, loading, router]);

  return (
    <div className='flex justify-center items-center h-64'>
      <LoadingSpinner size='large' />
    </div>
  );
}
