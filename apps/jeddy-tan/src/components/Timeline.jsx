import {
  VerticalTimeline,
  VerticalTimelineElement,
} from "react-vertical-timeline-component";
import "react-vertical-timeline-component/style.min.css";
import SchoolIcon from "@mui/icons-material/School";
import WorkIcon from "@mui/icons-material/Work";
import Experiences from "../data.json";

const parseStartDate = (dateStr) => new Date(dateStr.split(" - ")[0]);

const sortedExperiences = [...Experiences.experiences].sort((a, b) => {
  if (a.isActive !== b.isActive) return a.isActive ? -1 : 1;
  return parseStartDate(b.date) - parseStartDate(a.date);
});

function Timeline() {
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
            <h3 className="vertical-timeline-element-title">{item.title}</h3>
            {item.subtitle && (
              <h4 className="vertical-timeline-element-subtitle">
                {item.subtitle}
              </h4>
            )}
            {item.sections.map((section, i) => (
              <div key={i}>
                {section.heading && (
                  <p>
                    <strong>{section.heading}</strong>
                  </p>
                )}
                <ul>
                  {section.bullets.map((bullet, j) => (
                    <li key={j}>{bullet}</li>
                  ))}
                </ul>
              </div>
            ))}
          </VerticalTimelineElement>
        ))}
      </VerticalTimeline>
    </div>
  );
}

export default Timeline;
