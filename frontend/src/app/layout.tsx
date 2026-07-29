import type { Metadata } from "next";
import "@fontsource-variable/ibm-plex-sans/index.css";
import "@fontsource-variable/newsreader/index.css";
import "katex/dist/katex.min.css";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "NoteFoundry — Personal learning workspace",
    template: "%s — NoteFoundry"
  },
  description: "A single-owner workspace for dependable learning notes."
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
