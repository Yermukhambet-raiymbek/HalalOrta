import { Link } from "react-router";

/** Site footer: where the lesson content comes from, and the pricing link. */
export default function Footer() {
  return (
    <footer className="site-footer">
      <p>
        Lessons are based on guidance from the American Academy of Pediatrics and the CDC, with a link to the source
        in every lesson. LittleVillage is not medical advice: ask your pediatrician about your baby.
      </p>
      <p>
        <Link to="/pricing">Pricing</Link>
      </p>
    </footer>
  );
}
