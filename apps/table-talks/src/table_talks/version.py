"""Version and changelog parsing utilities."""

import json
import logging
import re
from pathlib import Path

logger = logging.getLogger(__name__)


def get_version() -> str:
    """Read version from package.json (the manifest `nx release` manages).

    Returns:
        Version string (e.g. "1.5.0") or "unknown" if not found.
    """
    try:
        package_json_path = Path(__file__).parent.parent.parent / "package.json"
        if not package_json_path.exists():
            logger.warning("package.json not found at %s", package_json_path)
            return "unknown"

        version = json.loads(package_json_path.read_text(encoding="utf-8")).get("version")
        if version:
            logger.info("Read version from package.json: %s", version)
            return version

        logger.warning("Version not found in package.json")
        return "unknown"
    except Exception as e:
        logger.exception("Failed to read version from package.json: %s", e)
        return "unknown"


def get_changelog(num_versions: int = 2) -> str:
    """Extract last N versions from CHANGELOG.md.

    Args:
        num_versions: Number of recent versions to extract (default: 2)

    Returns:
        Formatted changelog string with the last N versions, or a message if not available.
    """
    try:
        changelog_path = Path(__file__).parent.parent.parent / "CHANGELOG.md"
        if not changelog_path.exists():
            logger.warning("CHANGELOG.md not found at %s", changelog_path)
            return "Not available yet"

        content = changelog_path.read_text(encoding="utf-8")

        # Find all version sections (e.g. ## v1.0.1, ## 1.0.1, ## [1.0.1]).
        # `nx release`'s changelog format uses a single "#" for the newest
        # entry and demotes older ones to "##" as new releases land, so
        # both header levels need to match here.
        version_pattern = re.compile(
            r"^#{1,2}\s+(?:\[?v?(\d+\.\d+\.\d+)\]?.*?)$", re.MULTILINE | re.IGNORECASE
        )

        matches = list(version_pattern.finditer(content))

        if not matches:
            logger.warning("No version sections found in CHANGELOG.md")
            return "No releases yet"

        # Extract last N versions
        versions_to_show = matches[:num_versions]

        if not versions_to_show:
            return "No releases yet"

        # Extract content for each version
        changelog_sections: list[str] = []
        for i, match in enumerate(versions_to_show):
            version = match.group(1)
            start_pos = match.start()

            # Find end position (start of next version or end of file)
            if i + 1 < len(versions_to_show):
                end_pos = versions_to_show[i + 1].start()
            else:
                # Check if there's another version section after this one
                next_match_idx = matches.index(match) + 1
                if next_match_idx < len(matches):
                    end_pos = matches[next_match_idx].start()
                else:
                    end_pos = len(content)

            section = content[start_pos:end_pos].strip()

            # Clean up the section:
            # - Remove the version header (##)
            # - Format for Telegram display
            # - Limit to reasonable length (e.g., 500 chars per version)
            lines = section.split("\n")[1:]  # Skip the version header line
            section_content = "\n".join(lines).strip()

            # Format for Telegram:
            # Convert markdown headers (### Features) to bold (**Features**)
            section_content = re.sub(r"^###\s+(.+)$", r"*\1*", section_content, flags=re.MULTILINE)

            # Clean up list formatting (keep bullets but ensure spacing)
            section_content = re.sub(r"^-\s+", r"• ", section_content, flags=re.MULTILINE)

            # Truncate if too long
            max_chars = 500
            if len(section_content) > max_chars:
                section_content = section_content[:max_chars].rsplit("\n", 1)[0] + "\n..."

            changelog_sections.append(f"*v{version}*\n{section_content}")

        result = "\n\n".join(changelog_sections)
        logger.info("Extracted %d version(s) from CHANGELOG.md", len(changelog_sections))
        return result if result else "No release notes available"

    except Exception as e:
        logger.exception("Failed to parse CHANGELOG.md: %s", e)
        return "Error reading changelog"
