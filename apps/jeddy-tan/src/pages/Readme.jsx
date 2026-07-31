import "../styles/Readme.css";

// PLACEHOLDER — this is a draft. Replace with your own real values/working
// style before shipping; this is not meant to go out as final content.
const principles = [
  {
    title: "Ownership",
    description:
      "I treat problems as mine to solve end-to-end, not just my slice of them.",
  },
  {
    title: "Clarity over cleverness",
    description:
      "I'd rather ship something simple and understandable than something impressive and opaque.",
  },
  {
    title: "Continuous learning",
    description:
      "I actively seek out unfamiliar problems, because that's where the real growth is.",
  },
];

function Readme() {
  return (
    <div className="readme">
      <p className="readme-path">~/jeddy-tan/README.md</p>
      <h2># How I Work</h2>
      <p className="readme-intro">
        The short version of how I approach my work, in case you don&apos;t have
        time to read my whole timeline.
      </p>
      {principles.map((principle) => (
        <div className="readme-entry" key={principle.title}>
          <h3>## {principle.title}</h3>
          <p>{principle.description}</p>
        </div>
      ))}
    </div>
  );
}

export default Readme;
