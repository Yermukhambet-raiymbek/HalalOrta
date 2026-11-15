import { Link } from "react-router";

export default function NotFoundPage() {
  return (
    <section className="card">
      <h1>Page not found</h1>
      <p>There is nothing at this address.</p>
      <Link to="/">Go to the start page</Link>
    </section>
  );
}
