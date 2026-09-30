// The three post visibility tiers. Shared by the post form and every
// privacy badge so the wording shown to users can't drift between them -
// and it mirrors postVisibilityClause on the backend, which enforces it.
export const PRIVACY_OPTIONS = [
  {
    value: "public",
    icon: "globe",
    label: "Public",
    info: "All users in the social network will be able to see the post.",
  },
  {
    value: "almost_private",
    icon: "users",
    label: "Almost Private",
    info: "Only followers of the creator of the post will be able to see the post.",
  },
  {
    value: "private",
    icon: "lock",
    label: "Private",
    info: "Only the followers chosen by the creator of the post will be able to see it.",
  },
];

// A group post ignores privacy_level entirely - only group members can see
// it - so it gets its own badge instead of a misleading "Public" one.
export const GROUP_PRIVACY = {
  icon: "group",
  label: "Group",
  info: "Only members of this group will be able to see the post.",
};

export function getPostPrivacy(post) {
  if (post.group_id) return GROUP_PRIVACY;
  return PRIVACY_OPTIONS.find((o) => o.value === post.privacy_level) || null;
}
