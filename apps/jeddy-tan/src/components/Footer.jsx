import GitHubIcon from "@mui/icons-material/GitHub";
import LinkedInIcon from "@mui/icons-material/LinkedIn";
import "../styles/Footer.css";

function Footer() {
  return (
    <div className="footer">
      <div className="socialMedia">
        <a href="https://github.com/tanjd">
          <GitHubIcon />
        </a>
        <a href="https://www.linkedin.com/in/tanjeddy/">
          <LinkedInIcon />
        </a>
      </div>
      <p> &copy; 2026 jeddy-tan.com</p>
    </div>
  );
}

export default Footer;
