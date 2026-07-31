#!/usr/bin/env python3
"""Migrate existing CSV data to Google Sheets format.

This script converts the normalized CSV structure (separate themes.csv and questions.csv)
into a denormalized format suitable for Google Sheets (single sheet with all columns).

Usage:
    python scripts/csv_to_sheets.py
    python scripts/csv_to_sheets.py --output custom_output.csv

The output CSV can then be imported into Google Sheets.
"""

import argparse
import csv
import sys
from pathlib import Path


def migrate(data_dir: Path, output_file: Path) -> int:
    """Migrate CSV data to Google Sheets format.

    Args:
        data_dir: Directory containing themes.csv and questions.csv
        output_file: Path to write the output CSV

    Returns:
        Number of rows written (excluding header)
    """
    themes_csv = data_dir / "themes.csv"
    questions_csv = data_dir / "questions.csv"

    # Validate input files exist
    if not themes_csv.exists():
        print(f"Error: {themes_csv} not found", file=sys.stderr)
        sys.exit(1)
    if not questions_csv.exists():
        print(f"Error: {questions_csv} not found", file=sys.stderr)
        sys.exit(1)

    # Load themes into a dictionary
    themes = {}
    print(f"Reading themes from {themes_csv}...")
    with open(themes_csv, encoding="utf-8") as f:
        for row in csv.DictReader(f):
            theme_id = row.get("id", "").strip()
            if theme_id:
                themes[theme_id] = {
                    "label": row.get("label", "").strip(),
                    "description": row.get("description", "").strip(),
                }
    print(f"  Found {len(themes)} themes")

    # Load questions and merge with theme data
    output_rows = []
    print(f"Reading questions from {questions_csv}...")
    with open(questions_csv, encoding="utf-8") as f:
        for row in csv.DictReader(f):
            theme_id = row.get("theme_id", "").strip()
            question = row.get("question", "").strip()

            if not theme_id or not question:
                continue

            if theme_id not in themes:
                print(
                    f"  Warning: Question has unknown theme_id '{theme_id}', skipping",
                    file=sys.stderr,
                )
                continue

            output_rows.append(
                {
                    "theme_id": theme_id,
                    "theme_label": themes[theme_id]["label"],
                    "theme_description": themes[theme_id]["description"],
                    "question": question,
                }
            )
    print(f"  Found {len(output_rows)} questions")

    # Write output
    print(f"Writing denormalized data to {output_file}...")
    with open(output_file, "w", encoding="utf-8", newline="") as f:
        fieldnames = ["theme_id", "theme_label", "theme_description", "question"]
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(output_rows)

    print(f"✅ Successfully wrote {len(output_rows)} rows to {output_file}")
    return len(output_rows)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="Convert CSV data to Google Sheets format",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Next steps after running this script:
1. Create a new Google Sheet in your Google Drive
2. Import the generated CSV file (File > Import > Upload)
3. Share the sheet with your service account email (found in service account JSON)
4. Give the service account "Editor" permissions
5. Copy the sheet ID from the URL
6. Set GOOGLE_SHEET_ID in your .env file
7. Set GOOGLE_SERVICE_ACCOUNT_FILE or GOOGLE_SERVICE_ACCOUNT_JSON in .env
8. Set ENABLE_GOOGLE_SHEETS=true in .env
9. Restart your bot

For more information, see GOOGLE_SHEETS_PLAN.md
""",
    )
    parser.add_argument(
        "--output",
        "-o",
        default="data/sheets_template.csv",
        help="Output CSV file path (default: data/sheets_template.csv)",
    )
    parser.add_argument(
        "--data-dir",
        default="data",
        help="Directory containing themes.csv and questions.csv (default: data)",
    )
    args = parser.parse_args()

    # Resolve paths
    script_dir = Path(__file__).parent
    project_root = script_dir.parent
    data_dir = project_root / args.data_dir
    output_file = project_root / args.output

    # Create output directory if needed
    output_file.parent.mkdir(parents=True, exist_ok=True)

    # Run migration
    try:
        migrate(data_dir, output_file)
        print(f"\n{'=' * 70}")
        print("Migration complete!")
        print(f"{'=' * 70}")
        return 0
    except Exception as e:
        print(f"\n❌ Migration failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
