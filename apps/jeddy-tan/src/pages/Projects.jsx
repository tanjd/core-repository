import { useState } from "react";
import PropTypes from "prop-types";
import {
  Card,
  CardContent,
  Chip,
  Collapse,
  Grid,
  IconButton,
  Tooltip,
} from "@mui/material";
import GitHubIcon from "@mui/icons-material/GitHub";
import LaunchIcon from "@mui/icons-material/Launch";
import LightbulbIcon from "@mui/icons-material/Lightbulb";
import TelegramIcon from "@mui/icons-material/Telegram";
import projectsData from "../projectsData.json";
import "../styles/Projects.css";

function ProjectCard({ project }) {
  const [eli5Open, setEli5Open] = useState(false);
  const isTelegramLink = project.links?.live?.startsWith("https://t.me/");
  const LiveIcon = isTelegramLink ? TelegramIcon : LaunchIcon;

  return (
    <Card className="project-card">
      <CardContent>
        <div className="project-card-header">
          <h3 className="project-card-title">{project.name}</h3>
          {project.eli5 && (
            <Tooltip title="Explain like I'm 5" enterTouchDelay={0}>
              <IconButton
                size="small"
                className="eli5-toggle"
                aria-label="Toggle plain-language explanation"
                aria-pressed={eli5Open}
                onClick={() => setEli5Open((open) => !open)}
              >
                <LightbulbIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          )}
        </div>

        {project.eli5 && (
          <Collapse in={eli5Open} timeout="auto" unmountOnExit>
            <p className="eli5-text">
              <em>{project.eli5}</em>
            </p>
          </Collapse>
        )}
        <Collapse in={!eli5Open} timeout="auto" unmountOnExit>
          <p className="project-card-description">{project.description}</p>
        </Collapse>

        {project.tags?.length > 0 && (
          <div className="project-tags">
            {project.tags.map((tag) => (
              <Chip key={tag} label={tag} size="small" />
            ))}
          </div>
        )}

        {(project.links?.github || project.links?.live) && (
          <div className="project-links">
            {project.links?.github && (
              <a
                href={project.links.github}
                target="_blank"
                rel="noreferrer"
                aria-label={`${project.name} on GitHub`}
              >
                <GitHubIcon />
              </a>
            )}
            {project.links?.live && (
              <a
                href={project.links.live}
                target="_blank"
                rel="noreferrer"
                aria-label={`Try ${project.name}`}
              >
                <LiveIcon />
              </a>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

ProjectCard.propTypes = {
  project: PropTypes.shape({
    name: PropTypes.string.isRequired,
    description: PropTypes.string.isRequired,
    eli5: PropTypes.string,
    tags: PropTypes.arrayOf(PropTypes.string),
    links: PropTypes.shape({
      github: PropTypes.string,
      live: PropTypes.string,
    }),
  }).isRequired,
};

function Projects() {
  return (
    <div className="projects">
      <h2>Side Projects</h2>
      <p className="projects-intro">
        A few things I&apos;ve built outside of work, mostly to solve a problem
        I actually ran into.
      </p>
      <p className="projects-hint">
        Click the 💡 on any card for a plain-language explanation.
      </p>
      <Grid container spacing={3}>
        {projectsData.projects.map((project) => (
          <Grid item xs={12} sm={6} md={4} key={project.name}>
            <ProjectCard project={project} />
          </Grid>
        ))}
      </Grid>
    </div>
  );
}

export default Projects;
