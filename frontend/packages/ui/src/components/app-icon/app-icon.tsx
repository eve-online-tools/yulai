import { Icon, iconClass, type IconName } from "@xaroth.nl/design/react";

// Tabler Icons (MIT) outline paths the design package does not ship.
const extra = {
  settings: [
    "M10.325 4.317c.426 -1.756 2.924 -1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543 -.94 3.31 .826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756 .426 1.756 2.924 0 3.35a1.724 1.724 0 0 0 -1.066 2.573c.94 1.543 -.826 3.31 -2.37 2.37a1.724 1.724 0 0 0 -2.572 1.065c-.426 1.756 -2.924 1.756 -3.35 0a1.724 1.724 0 0 0 -2.573 -1.066c-1.543 .94 -3.31 -.826 -2.37 -2.37a1.724 1.724 0 0 0 -1.065 -2.572c-1.756 -.426 -1.756 -2.924 0 -3.35a1.724 1.724 0 0 0 1.066 -2.573c-.94 -1.543 .826 -3.31 2.37 -2.37c1 .608 2.296 .07 2.572 -1.065z",
    "M9 12a3 3 0 1 0 6 0a3 3 0 0 0 -6 0",
  ],
  users: [
    "M5 7a4 4 0 1 0 8 0a4 4 0 1 0 -8 0",
    "M3 21v-2a4 4 0 0 1 4 -4h4a4 4 0 0 1 4 4v2",
    "M16 3.13a4 4 0 0 1 0 7.75",
    "M21 21v-2a4 4 0 0 0 -3 -3.85",
  ],
  map: ["M3 7l6 -3l6 3l6 -3v13l-6 3l-6 -3l-6 3v-13", "M9 4v13", "M15 7v13"],
  ship: [
    "M4 13a8 8 0 0 1 7 7a6 6 0 0 0 3 -5a9 9 0 0 0 6 -8a3 3 0 0 0 -3 -3a9 9 0 0 0 -8 6a6 6 0 0 0 -5 3",
    "M7 14a6 6 0 0 0 -3 6a6 6 0 0 0 6 -3",
    "M14 9a1 1 0 1 0 2 0a1 1 0 1 0 -2 0",
  ],
  checklist: ["M3.5 5.5l1.5 1.5l2.5 -2.5", "M3.5 11.5l1.5 1.5l2.5 -2.5", "M3.5 17.5l1.5 1.5l2.5 -2.5", "M11 6l9 0", "M11 12l9 0", "M11 18l9 0"],
} as const;

export type AppIconName = IconName | keyof typeof extra;

export function AppIcon({ name, size }: { name: AppIconName; size?: number }) {
  if (!(name in extra)) return <Icon name={name as IconName} size={size} />;
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      width={size}
      height={size}
      aria-hidden="true"
      className={iconClass()}
    >
      {extra[name as keyof typeof extra].map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}
