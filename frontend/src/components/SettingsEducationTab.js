"use client";

import { useState, useEffect } from "react";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";

// Kept in sync by hand with educationProviderOptions/qualificationNameOptions
// /rqfLevelOptions in backend/pkg/handlers/user.go, which validates a saved
// selection against the same three lists.
const EDUCATION_PROVIDER_OPTIONS = [
  "Secondary School",
  "Sixth Form",
  "Further Education (FE) College",
  "University",
  "Higher Education Institution",
  "Independent Training Provider (ITPs)",
  "Employer Provider",
];

const QUALIFICATION_NAME_OPTIONS = [
  "GCSE",
  "A Level",
  "Bachelor’s Degree",
  "Master’s Degree",
  "PhD",
  "Doctorate",
  "BTEC",
  "NVQ",
  "T Level",
  "V Level",
  "Higher Technical Qualification (HTQ)",
  "Access to HE Diploma",
  "Apprenticeship",
];

const RQF_LEVEL_OPTIONS = [
  "Entry Level",
  "Level 1",
  "Level 2",
  "Level 3",
  "Level 4",
  "Level 5",
  "Level 6",
  "Level 7",
  "Level 8",
];

// The "Education" tab of the Settings page: the educational body/provider,
// qualification, and RQF level a user studies or trained under.
export default function SettingsEducationTab({
  user,
  isLoading,
  setIsLoading,
  setMessage,
}) {
  const [education, setEducation] = useState({
    educationProvider: "",
    qualificationName: "",
    rqfLevel: "",
  });
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!user) return;

    const loadEducation = async () => {
      try {
        const { ok, data } = await apiFetch(`/user/profile?id=${user.id}`);
        if (ok) {
          setEducation({
            educationProvider: data.education_provider || "",
            qualificationName: data.qualification_name || "",
            rqfLevel: data.rqf_level || "",
          });
        } else {
          toast.error("Failed to load education details");
        }
      } catch (error) {
        console.error("Error loading education details:", error);
        toast.error("Failed to load education details");
      } finally {
        setLoaded(true);
      }
    };

    loadEducation();
  }, [user]);

  const handleChange = (e) => {
    const { name, value } = e.target;
    setEducation({ ...education, [name]: value });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsLoading(true);
    setMessage("");

    try {
      const { ok, data } = await apiFetch("/user/education", {
        method: "PUT",
        json: {
          education_provider: education.educationProvider,
          qualification_name: education.qualificationName,
          rqf_level: education.rqfLevel,
        },
      });

      if (!ok) {
        setMessage(data.error || "Failed to update education details");
        return;
      }

      setMessage("Education details updated successfully!");
    } catch (error) {
      console.error("Error updating education details:", error);
      setMessage("Failed to update education details");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className='space-y-4'>
      <h2 className='text-lg font-semibold'>Education</h2>
      <p className='text-sm text-gray-500'>
        Tell us about the institution and qualification you are studying
      </p>

      <div>
        <label
          htmlFor='settings-education-provider'
          className='block text-sm font-medium text-gray-700 mb-1'>
          Educational Body (Provider)
        </label>
        <select
          id='settings-education-provider'
          name='educationProvider'
          value={education.educationProvider}
          onChange={handleChange}
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'>
          <option value=''>Select a provider...</option>
          {EDUCATION_PROVIDER_OPTIONS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label
          htmlFor='settings-qualification-name'
          className='block text-sm font-medium text-gray-700 mb-1'>
          Qualification Name
        </label>
        <select
          id='settings-qualification-name'
          name='qualificationName'
          value={education.qualificationName}
          onChange={handleChange}
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'>
          <option value=''>Select a qualification...</option>
          {QUALIFICATION_NAME_OPTIONS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>

      <div>
        <label
          htmlFor='settings-rqf-level'
          className='block text-sm font-medium text-gray-700 mb-1'>
          Regulated Level (RQF)
        </label>
        <select
          id='settings-rqf-level'
          name='rqfLevel'
          value={education.rqfLevel}
          onChange={handleChange}
          className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'>
          <option value=''>Select a level...</option>
          {RQF_LEVEL_OPTIONS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </div>

      <button
        type='submit'
        disabled={isLoading || !loaded}
        className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
        {isLoading ? "Saving..." : !loaded ? "Loading..." : "Save Education Details"}
      </button>
    </form>
  );
}
