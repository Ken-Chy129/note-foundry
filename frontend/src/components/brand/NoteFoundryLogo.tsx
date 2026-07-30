interface NoteFoundryLogoProps {
  className?: string;
}

export function NoteFoundryLogo({ className }: NoteFoundryLogoProps) {
  return (
    <svg
      className={className ? `note-foundry-logo ${className}` : "note-foundry-logo"}
      viewBox="0 0 36 36"
      aria-hidden="true"
      focusable="false"
    >
      <path className="logo-sheet" d="M7 3.5h14.5L29 11v21.5H7z" />
      <path className="logo-fold" d="M21.5 3.5V11H29z" />
      <path className="logo-monogram" d="M12 26V11.5L24 26V11.5" />
      <path className="logo-spark" d="m29.5 2 .85 2.15L32.5 5l-2.15.85L29.5 8l-.85-2.15L26.5 5l2.15-.85z" />
    </svg>
  );
}
