import GitHubIcon from "@mui/icons-material/GitHub";
import LinkedInIcon from "@mui/icons-material/LinkedIn";
import "../styles/Home.css";
import Timeline from "../components/Timeline";

function Home() {
  return (
    <div className="home">
      <div className="about">
        <h2> Hello! I am Jeddy.</h2>
        <div className="prompt">
          <p>
            I am passionate about creating value added solutions with
            like-minded people
          </p>
          <a href="https://github.com/tanjd">
            <GitHubIcon />
          </a>
          <a href="https://www.linkedin.com/in/tanjeddy/">
            <LinkedInIcon />
          </a>
        </div>
      </div>
      <Timeline />
    </div>
  );
}

export default Home;
