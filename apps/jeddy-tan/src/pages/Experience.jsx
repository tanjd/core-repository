import "../styles/Experience.css";
import Timeline from "../components/Timeline";

function Experience() {
  return (
    <div className="experience">
      <p className="experience-hint">
        Click the 💡 next to any project for a plain-language explanation.
      </p>
      <Timeline variant="full" />
    </div>
  );
}

export default Experience;
