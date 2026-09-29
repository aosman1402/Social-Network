"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/context/AuthContext";
import GitHubLoginButton from "@/components/GitHubLoginButton";
import LoginVideo from "@/components/LoginVideo";

// Messages for the ?oauth_error= codes the backend's GitHub login sends
// back here (backend/pkg/handlers/oauth_github.go).
const OAUTH_ERRORS = {
  denied: "GitHub login was cancelled.",
  invalid_state: "That GitHub login link expired or was invalid - please try again.",
  github_error: "GitHub didn't accept the login - please try again.",
  no_verified_email: "Your GitHub account needs a verified email address to log in here.",
  not_configured: "GitHub login isn't set up on this server.",
  server_error: "Something went wrong logging in with GitHub - please try again.",
};

export default function LoginPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  // Read from window.location rather than useSearchParams(), which would
  // need a Suspense boundary for this page to build.
  useEffect(() => {
    const code = new URLSearchParams(window.location.search).get("oauth_error");
    if (code) setError(OAUTH_ERRORS[code] || OAUTH_ERRORS.server_error);
  }, []);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setIsLoading(true);

    if (!email || !password) {
      setError("Please fill in all fields");
      setIsLoading(false);
      return;
    }

    try {
      const result = await login(email, password);

      if (result.success) {
        // ✅ Force a full page refresh to ensure navbar updates
        window.location.href = "/";
      } else {
        setError(result.error || "Login failed. Please try again.");
      }
    } catch (error) {
      console.error("Login error:", error);
      setError("Login failed. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className='login-landing'>
      <LoginVideo />
      <div className='max-w-md w-full bg-white rounded-lg shadow-lg p-8'>
        <h2 className='text-3xl font-bold text-center mb-6'>Login</h2>

        <form onSubmit={handleSubmit} className='space-y-4'>
          <div>
            <label
              htmlFor='login-email'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Email
            </label>
            <input
              id='login-email'
              name='email'
              type='email'
              autoComplete='email'
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
              placeholder='Enter your email'
              required
            />
          </div>

          <div>
            <label
              htmlFor='login-password'
              className='block text-sm font-medium text-gray-700 mb-1'>
              Password
            </label>
            <input
              id='login-password'
              name='password'
              type='password'
              autoComplete='current-password'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
              placeholder='Enter your password'
              required
            />
          </div>

          {error && (
            <div className='text-red-500 text-sm text-center'>{error}</div>
          )}

          <button
            type='submit'
            disabled={isLoading}
            className='w-full bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 transition disabled:opacity-50'>
            {isLoading ? "Logging in..." : "Login"}
          </button>
        </form>

        <GitHubLoginButton />

        <p className='text-center mt-4 text-sm'>
          Don't have an account?{" "}
          <Link href='/register' className='text-blue-600 hover:underline'>
            Register
          </Link>
        </p>
      </div>
    </div>
  );
}
