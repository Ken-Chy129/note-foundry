import Link from "next/link";

export default function NotFound() {
  return (
    <main className="not-found">
      <p className="section-kicker">404 / Missing page</p>
      <h1>This note is not public.</h1>
      <p>It may be private, still a draft, in Trash, or no longer available at this address.</p>
      <Link className="button button-primary" href="/">Browse public knowledge</Link>
    </main>
  );
}
