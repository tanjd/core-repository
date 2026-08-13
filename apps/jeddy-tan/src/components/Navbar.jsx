import { useEffect, useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import PropTypes from "prop-types";
import "../styles/Navbar.css";
import ReorderIcon from "@mui/icons-material/Reorder";
import CloseIcon from "@mui/icons-material/Close";

const NAV_LINKS = [
  { to: "/", label: "home" },
  { to: "/projects", label: "projects" },
  { to: "/experience", label: "experience" },
];

function NavItem({ to, label, mobile, onClick }) {
  return (
    <NavLink
      to={to}
      end
      onClick={onClick}
      className={({ isActive }) =>
        "navbar-link" +
        (mobile ? " navbar-link--mobile" : "") +
        (isActive ? " navbar-link--active" : "")
      }
    >
      <span className="navbar-link-prompt">$ </span>
      {label}
      <span className="navbar-cursor" aria-hidden="true">
        █
      </span>
    </NavLink>
  );
}

NavItem.propTypes = {
  to: PropTypes.string.isRequired,
  label: PropTypes.string.isRequired,
  mobile: PropTypes.bool,
  onClick: PropTypes.func,
};

function Navbar() {
  const [isOpen, setIsOpen] = useState(false);
  const location = useLocation();

  useEffect(() => {
    setIsOpen(false);
  }, [location]);

  useEffect(() => {
    if (!isOpen) return;
    document.body.style.overflow = "hidden";
    const handleKeyDown = (event) => {
      if (event.key === "Escape") setIsOpen(false);
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = "";
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  return (
    <nav className="navbar">
      <div className="navbar-bar">
        <NavLink to="/" end className="navbar-brand">
          ~/jeddy-tan
        </NavLink>

        <div className="navbar-links">
          {NAV_LINKS.map((link) => (
            <NavItem key={link.to} {...link} />
          ))}
        </div>

        <button
          type="button"
          className="navbar-toggle"
          aria-label={isOpen ? "Close menu" : "Open menu"}
          aria-expanded={isOpen}
          aria-controls="navbar-mobile-menu"
          onClick={() => setIsOpen((prev) => !prev)}
        >
          {isOpen ? <CloseIcon /> : <ReorderIcon />}
        </button>
      </div>

      <div
        id="navbar-mobile-menu"
        className={"navbar-overlay" + (isOpen ? " navbar-overlay--open" : "")}
        aria-hidden={!isOpen}
      >
        {NAV_LINKS.map((link) => (
          <NavItem
            key={link.to}
            {...link}
            mobile
            onClick={() => setIsOpen(false)}
          />
        ))}
      </div>
    </nav>
  );
}

export default Navbar;
