"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/context/AuthContext";

export default function RegisterPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [formData, setFormData] = useState({
    email: "",
    password: "",
    confirmPassword: "",
    first_name: "",
    last_name: "",
    date_of_birth: "",
    nickname: "",
    about_me: "",
  });
  const [errors, setErrors] = useState({});
  const [isLoading, setIsLoading] = useState(false);
  const [serverError, setServerError] = useState("");
  const [debugInfo, setDebugInfo] = useState("");

  const validateForm = () => {
    const newErrors = {};
    if (!formData.email) newErrors.email = "Email is required";
    else if (!/\S+@\S+\.\S+/.test(formData.email))
      newErrors.email = "Please enter a valid email";
    if (!formData.password) newErrors.password = "Password is required";
    else if (formData.password.length < 8)
      newErrors.password = "Password must be at least 8 characters";
    if (formData.password !== formData.confirmPassword)
      newErrors.confirmPassword = "Passwords do not match";
    if (!formData.first_name) newErrors.first_name = "First name is required";
    if (!formData.last_name) newErrors.last_name = "Last name is required";
    if (!formData.date_of_birth)
      newErrors.date_of_birth = "Date of birth is required";
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData({ ...formData, [name]: value });
    if (errors[name]) setErrors({ ...errors, [name]: "" });
    setServerError("");
    setDebugInfo("");
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!validateForm()) return;

    setIsLoading(true);
    setServerError("");
    setDebugInfo("");

    try {
      setDebugInfo("1. Formatting date...");
      // Format date from DD/MM/YYYY to YYYY-MM-DD
      let formattedDate = formData.date_of_birth;
      if (formData.date_of_birth.includes("/")) {
        const parts = formData.date_of_birth.split("/");
        formattedDate = `${parts[2]}-${parts[1].padStart(2, "0")}-${parts[0].padStart(2, "0")}`;
      }

      const payload = {
        email: formData.email,
        password: formData.password,
        first_name: formData.first_name,
        last_name: formData.last_name,
        date_of_birth: formattedDate,
        nickname: formData.nickname || null,
        about_me: formData.about_me || null,
        avatar_path: null,
      };

      setDebugInfo("2. Sending registration request...");
      console.log("Sending registration:", payload);

      const response = await fetch("http://localhost:8081/api/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(payload),
      });

      setDebugInfo(`3. Registration response status: ${response.status}`);
      console.log("Registration response status:", response.status);

      const data = await response.json();
      setDebugInfo(`4. Registration response: ${JSON.stringify(data)}`);
      console.log("Registration response:", data);

      if (!response.ok) {
        throw new Error(data.message || data.error || "Registration failed");
      }

      setDebugInfo("5. Registration successful! Auto-logging in...");
      console.log("Registration successful! Auto-logging in...");

      // ✅ Try to auto-login
      const loginResult = await login(formData.email, formData.password);
      console.log("Auto-login result:", loginResult);

      if (loginResult.success) {
        setDebugInfo("6. Auto-login successful! Redirecting to home...");
        console.log("Auto-login successful! Redirecting to home...");
        // Force redirect with window.location
        window.location.href = "/";
      } else {
        setDebugInfo(`6. Auto-login failed: ${loginResult.error}`);
        console.error("Auto-login failed:", loginResult.error);
        // Fallback: redirect to login page
        router.push("/login?registered=true");
      }
    } catch (error) {
      console.error("Registration error:", error);
      setServerError(error.message || "Registration failed. Please try again.");
      setDebugInfo(`❌ Error: ${error.message}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className='min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8'>
      <div className='max-w-md w-full bg-white rounded-lg shadow-lg p-8'>
        <div className='text-center mb-8'>
          <h2 className='text-3xl font-bold text-gray-900'>Create Account</h2>
          <p className='text-gray-600 mt-2'>Join the social network</p>
        </div>

        <form onSubmit={handleSubmit} className='space-y-4'>
          <div>
            <label className='block text-sm font-medium text-gray-700 mb-1'>
              Email *
            </label>
            <input
              type='email'
              name='email'
              value={formData.email}
              onChange={handleChange}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.email ? "border-red-500" : "border-gray-300"
              }`}
              placeholder='Enter your email'
            />
            {errors.email && (
              <p className='text-red-500 text-sm mt-1'>{errors.email}</p>
            )}
          </div>

          <div className='grid grid-cols-2 gap-4'>
            <div>
              <label className='block text-sm font-medium text-gray-700 mb-1'>
                First Name *
              </label>
              <input
                type='text'
                name='first_name'
                value={formData.first_name}
                onChange={handleChange}
                className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                  errors.first_name ? "border-red-500" : "border-gray-300"
                }`}
                placeholder='First name'
              />
              {errors.first_name && (
                <p className='text-red-500 text-sm mt-1'>{errors.first_name}</p>
              )}
            </div>
            <div>
              <label className='block text-sm font-medium text-gray-700 mb-1'>
                Last Name *
              </label>
              <input
                type='text'
                name='last_name'
                value={formData.last_name}
                onChange={handleChange}
                className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                  errors.last_name ? "border-red-500" : "border-gray-300"
                }`}
                placeholder='Last name'
              />
              {errors.last_name && (
                <p className='text-red-500 text-sm mt-1'>{errors.last_name}</p>
              )}
            </div>
          </div>

          <div>
            <label className='block text-sm font-medium text-gray-700 mb-1'>
              Date of Birth * (DD/MM/YYYY)
            </label>
            <input
              type='text'
              name='date_of_birth'
              value={formData.date_of_birth}
              onChange={handleChange}
              placeholder='30/10/1998'
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.date_of_birth ? "border-red-500" : "border-gray-300"
              }`}
            />
            {errors.date_of_birth && (
              <p className='text-red-500 text-sm mt-1'>
                {errors.date_of_birth}
              </p>
            )}
          </div>

          <div>
            <label className='block text-sm font-medium text-gray-700 mb-1'>
              Password *
            </label>
            <input
              type='password'
              name='password'
              autoComplete='new-password'
              value={formData.password}
              onChange={handleChange}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.password ? "border-red-500" : "border-gray-300"
              }`}
              placeholder='Min 8 characters'
            />
            {errors.password && (
              <p className='text-red-500 text-sm mt-1'>{errors.password}</p>
            )}
          </div>

          <div>
            <label className='block text-sm font-medium text-gray-700 mb-1'>
              Confirm Password *
            </label>
            <input
              type='password'
              name='confirmPassword'
              autoComplete='new-password'
              value={formData.confirmPassword}
              onChange={handleChange}
              className={`w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                errors.confirmPassword ? "border-red-500" : "border-gray-300"
              }`}
              placeholder='Confirm your password'
            />
            {errors.confirmPassword && (
              <p className='text-red-500 text-sm mt-1'>
                {errors.confirmPassword}
              </p>
            )}
          </div>

          <div>
            <label className='block text-sm font-medium text-gray-700 mb-1'>
              Nickname <span className='text-gray-400 text-xs'>(Optional)</span>
            </label>
            <input
              type='text'
              name='nickname'
              value={formData.nickname}
              onChange={handleChange}
              className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
              placeholder='Choose a unique nickname'
            />
          </div>

          <div>
            <label className='block text-sm font-medium text-gray-700 mb-1'>
              About Me <span className='text-gray-400 text-xs'>(Optional)</span>
            </label>
            <textarea
              name='about_me'
              value={formData.about_me}
              onChange={handleChange}
              rows='3'
              className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
              placeholder='Tell us about yourself...'
            />
          </div>

          {debugInfo && (
            <div className='text-blue-500 text-xs text-center break-all'>
              {debugInfo}
            </div>
          )}

          {serverError && (
            <div className='text-red-500 text-sm text-center'>
              {serverError}
            </div>
          )}

          <button
            type='submit'
            disabled={isLoading}
            className='w-full bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition disabled:opacity-50 disabled:cursor-not-allowed'>
            {isLoading ? "Creating account..." : "Create Account"}
          </button>
        </form>

        <div className='mt-6 text-center'>
          <p className='text-sm text-gray-600'>
            Already have an account?{" "}
            <Link
              href='/login'
              className='text-blue-600 hover:text-blue-800 font-medium'>
              Sign in
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
