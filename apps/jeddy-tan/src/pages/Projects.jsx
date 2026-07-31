import { useState } from "react";
import PropTypes from "prop-types";
import {
  Card,
  CardContent,
  Chip,
  Collapse,
  Grid,
  IconButton,
  Typography,
} from "@mui/material";
import GitHubIcon from "@mui/icons-material/GitHub";
import LightbulbIcon from "@mui/icons-material/Lightbulb";
import projectsData from "../projectsData.json";
import "../styles/Projects.css";

function ProjectCard({ project }) {
  const [eli5Open, setEli5Open] = useState(false);

  return (
    <Card className="project-card">
      <CardContent>
        <div className="project-card-header">
          <Typography variant="h6">{project.name}</Typography>
          {project.eli5 && (
            <IconButton
              size="small"
              aria-label="Toggle plain-language explanation"
              onClick={() => setEli5Open((open) => !open)}
            >
              <LightbulbIcon fontSize="small" />
            </IconButton>
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
          <Typography variant="body2">{project.description}</Typography>
        </Collapse>

        {project.tags?.length > 0 && (
          <div className="project-tags">
            {project.tags.map((tag) => (
              <Chip key={tag} label={tag} size="small" />
            ))}
          </div>
        )}

        {project.links?.github && (
          <div className="project-links">
            <a href={project.links.github} target="_blank" rel="noreferrer">
              <GitHubIcon />
            </a>
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
