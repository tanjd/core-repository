import { useState } from "react";
import PropTypes from "prop-types";
import { Button, Collapse, IconButton, Tooltip } from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";
import LightbulbIcon from "@mui/icons-material/Lightbulb";

const sectionShape = PropTypes.shape({
  heading: PropTypes.string,
  eli5: PropTypes.string,
  bullets: PropTypes.arrayOf(PropTypes.string).isRequired,
});

function SectionDetail({ section }) {
  const [eli5Open, setEli5Open] = useState(false);

  return (
    <div className="timeline-section">
      <div className="timeline-section-header">
        {section.heading && <strong>{section.heading}</strong>}
        {section.eli5 && (
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
      {section.eli5 && (
        <Collapse in={eli5Open} timeout="auto" unmountOnExit>
          <p className="eli5-text">
            <em>{section.eli5}</em>
          </p>
        </Collapse>
      )}
      <Collapse in={!eli5Open} timeout="auto" unmountOnExit>
        <ul>
          {section.bullets.map((bullet, j) => (
            <li key={j}>{bullet}</li>
          ))}
        </ul>
      </Collapse>
    </div>
  );
}

function TimelineEntryContent({ item, variant = "full" }) {
  const [detailsOpen, setDetailsOpen] = useState(false);

  const highlights = item.highlights?.length
    ? item.highlights
    : [item.sections?.[0]?.bullets?.[0]].filter(Boolean);

  return (
    <>
      <h3 className="vertical-timeline-element-title">{item.title}</h3>
      {item.subtitle && (
        <h4 className="vertical-timeline-element-subtitle">{item.subtitle}</h4>
      )}

      {highlights.length > 0 && (
        <ul className="highlights-list">
          {highlights.map((highlight, i) => (
            <li key={i}>{highlight}</li>
          ))}
        </ul>
      )}

      {variant === "full" && item.sections?.length > 0 && (
        <>
          <Button
            size="small"
            onClick={() => setDetailsOpen((open) => !open)}
            endIcon={detailsOpen ? <ExpandLessIcon /> : <ExpandMoreIcon />}
          >
            {detailsOpen ? "Show less" : "Show more"}
          </Button>
          <Collapse in={detailsOpen} timeout="auto" unmountOnExit>
            {item.sections.map((section, i) => (
              <SectionDetail key={i} section={section} />
            ))}
          </Collapse>
        </>
      )}
    </>
  );
}

SectionDetail.propTypes = {
  section: sectionShape.isRequired,
};

TimelineEntryContent.propTypes = {
  item: PropTypes.shape({
    title: PropTypes.string.isRequired,
    subtitle: PropTypes.string,
    highlights: PropTypes.arrayOf(PropTypes.string),
    sections: PropTypes.arrayOf(sectionShape),
  }).isRequired,
  variant: PropTypes.oneOf(["full", "condensed"]),
};

export default TimelineEntryContent;
