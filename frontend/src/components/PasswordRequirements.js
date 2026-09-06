import { getPasswordChecks } from "@/utils/helpers";

const REQUIREMENTS = [
  { key: "length", label: "12 to 15 characters required" },
  { key: "lowercase", label: "One lowercase letter (a-z)" },
  { key: "uppercase", label: "One uppercase letter (A-Z)" },
  { key: "digit", label: "One number (0-9)" },
  { key: "special", label: "One special character (e.g. ! @ # $ %)" },
];

// Live checklist of the password policy, ticking each requirement off as
// the user types. Shared between Register and Settings (Change Password)
// so both enforce - and display - the exact same rule.
export default function PasswordRequirements({ password }) {
  const checks = getPasswordChecks(password);

  return (
    <ul className='text-xs mt-2 space-y-1'>
      {REQUIREMENTS.map((req) => (
        <li
          key={req.key}
          className={checks[req.key] ? "text-green-700" : "text-gray-500"}>
          {checks[req.key] ? "✓" : "○"} {req.label}
        </li>
      ))}
      {password && !checks.validCharsOnly && (
        <li className='text-red-500'>
          ✕ Contains a character that isn't allowed
        </li>
      )}
    </ul>
  );
}
