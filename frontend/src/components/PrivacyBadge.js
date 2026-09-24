"use client";

import InfoTooltip from "@/components/InfoTooltip";
import { getPostPrivacy } from "@/lib/privacy";

// Shows who can see a post - icon, tier name, and an "i" explaining it.
export default function PrivacyBadge({ post, className = "" }) {
  const privacy = getPostPrivacy(post);
  if (!privacy) return null;

  return (
    <span className={`privacy-badge text-xs px-2 py-0.5 rounded ${className}`}>
      <span aria-hidden='true'>{privacy.icon}</span>
      {privacy.label}
      <InfoTooltip text={privacy.info} />
    </span>
  );
}
