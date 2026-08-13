import "../styles/Readme.css";
import GitHubIcon from "@mui/icons-material/GitHub";
import LinkedInIcon from "@mui/icons-material/LinkedIn";
import HandshakeIcon from "@mui/icons-material/Handshake";
import FlagIcon from "@mui/icons-material/Flag";
import SchoolIcon from "@mui/icons-material/School";

const principles = [
  {
    title: "People centric",
    icon: HandshakeIcon,
    description:
      "I want to be someone people actually want on their team — reliable, kind, and genuinely fun to be around.",
    question: "Do people actually want to work with me?",
  },
  {
    title: "Wholeheartedly",
    icon: FlagIcon,
    description: "I try to work heartily, not to be recognized for it.",
    question: "Would I still give my best effort if no one ever rewarded it?",
  },
  {
    title: "Intellectual honesty",
    icon: SchoolIcon,
    description:
      "It's okay to say I don't know — I try not to pretend otherwise. 知之為知之，不知為不知，是知也: to know what you know, and what you don't.",
    question:
      "Would I still say 'I don't know' if it made me look less capable?",
  },
];

function Readme() {
  return (
    <div className="readme">
      <div className="readme-content">
        <p className="readme-path">~/jeddy-tan/README.md</p>
        <h1># jeddy-tan</h1>
        <p className="readme-intro">
          Hi, I&apos;m Jeddy — welcome to my corner of the internet.
        </p>
        <div className="readme-links">
          <a href="https://github.com/tanjd" aria-label="GitHub">
            <GitHubIcon />
          </a>
          <a href="https://www.linkedin.com/in/tanjeddy/" aria-label="LinkedIn">
            <LinkedInIcon />
          </a>
        </div>

        <h2>## How I Work</h2>
        <p className="readme-intro">
          These are values I aspire to live out, not ones I hold perfectly —
          writing them down here is as much a reminder to myself as it is
          anything else.
        </p>
        <div className="readme-entries">
          {principles.map(({ title, icon: Icon, description, question }) => (
            <div className="readme-entry" key={title}>
              <div className="readme-entry-header">
                <Icon className="readme-entry-icon" fontSize="small" />
                <h3>{title}</h3>
              </div>
              <p>{description}</p>
              <p className="readme-entry-question">{question}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default Readme;
