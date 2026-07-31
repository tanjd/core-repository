import PropTypes from "prop-types";
import {
  VerticalTimeline,
  VerticalTimelineElement,
} from "react-vertical-timeline-component";
import "react-vertical-timeline-component/style.min.css";
import SchoolIcon from "@mui/icons-material/School";
import WorkIcon from "@mui/icons-material/Work";
import Experiences from "../data.json";
import TimelineEntryContent from "./TimelineEntryContent";
import "../styles/Timeline.css";

const parseStartDate = (dateStr) => new Date(dateStr.split(" - ")[0]);

const sortedExperiences = [...Experiences.experiences].sort((a, b) => {
  if (a.isActive !== b.isActive) return a.isActive ? -1 : 1;
  return parseStartDate(b.date) - parseStartDate(a.date);
});

function Timeline({ variant = "full" }) {
  return (
    <div className="experience">
      <VerticalTimeline lineColor="#192428">
        {sortedExperiences.map((item, id) => (
          <VerticalTimelineElement
            key={id}
            className={
              item.type === "work"
                ? "vertical-timeline-element--work"
                : "vertical-timeline-element--education"
            }
            date={item.date}
            iconStyle={{
              background: item.isActive ? "#d8ab4e" : "#192428",
              color: item.isActive ? "#192428" : "#fff",
            }}
            icon={item.type === "work" ? <WorkIcon /> : <SchoolIcon />}
          >
            <TimelineEntryContent item={item} variant={variant} />
          </VerticalTimelineElement>
        ))}
      </VerticalTimeline>
    </div>
  );
}

Timeline.propTypes = {
  variant: PropTypes.oneOf(["full", "condensed"]),
};

export default Timeline;
