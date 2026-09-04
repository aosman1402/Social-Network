"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function ProfileRedirect() {
  const router = useRouter();

  useEffect(() => {
    const redirectToMyProfile = async () => {
      try {
        const response = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });
        if (response.ok) {
          const data = await response.json();
          if (data.id) {
            router.replace(`/profile/${data.id}`);
          } else {
            router.replace("/login");
          }
        } else {
          router.replace("/login");
        }
      } catch {
        router.replace("/login");
      }
    };

    redirectToMyProfile();
  }, [router]);

  return (
    <div className='flex justify-center items-center h-64'>
      <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
    </div>
  );
}