import { readFile, writeFile } from "fs/promises";
import { join } from "path";

const problematicFiles = [
  "Cordoba (Food).csv",
  "Romantic Road (Food).csv",
  "Seville (Food).csv",
  "Valencia (Food).csv",
];

async function fixCsvFile(filename: string) {
  const filePath = join(process.cwd(), "data/food-maps", filename);
  console.log(`\nFixing ${filename}...`);

  try {
    const content = await readFile(filePath, "utf-8");

    // First, normalize line endings
    const normalizedContent = content.replace(/\r\n/g, "\n");

    // Split into records more carefully to handle multi-line fields
    const records: string[][] = [];
    let currentRecord: string[] = [];
    let currentField = "";
    let inQuotes = false;

    normalizedContent.split("").forEach((char) => {
      if (char === '"') {
        inQuotes = !inQuotes;
        currentField += char;
      } else if (char === "," && !inQuotes) {
        currentRecord.push(currentField.trim());
        currentField = "";
      } else if (char === "\n" && !inQuotes) {
        currentRecord.push(currentField.trim());
        if (currentRecord.some((field) => field.length > 0)) {
          records.push(currentRecord);
        }
        currentRecord = [];
        currentField = "";
      } else {
        currentField += char;
      }
    });

    // Handle the last field and record
    if (currentField) {
      currentRecord.push(currentField.trim());
    }
    if (currentRecord.length > 0) {
      records.push(currentRecord);
    }

    // Convert records to properly formatted CSV lines
    const lines = records.map((record) => {
      if (record.length === 5) {
        return record
          .map((field) => {
            // Ensure proper quoting
            if (
              field.includes(",") ||
              field.includes('"') ||
              field.includes("\n")
            ) {
              return `"${field.replace(/"/g, '""')}"`;
            }
            return field;
          })
          .join(",");
      }
      // For records that aren't properly formatted
      const title = record.join(" ").trim();
      return `"${title.replace(/"/g, '""')}","Local food/drink item","","food,local",""`;
    });

    // If first line isn't the header, add it
    const hasHeader = lines[0] === "Title,Note,URL,Tags,Comment";
    if (!hasHeader) {
      lines.unshift("Title,Note,URL,Tags,Comment");
    }

    // For each non-header line that doesn't have enough columns,
    // convert it to a proper CSV row
    const fixedLines = lines.map((line, index) => {
      if (index === 0 && hasHeader) return line;

      const parts = line.split(",");
      if (parts.length === 5) return line;

      // It's a food item without proper structure
      // Escape any quotes in the line
      const escapedLine = line.replace(/"/g, '""');
      return `"${escapedLine}","Local food/drink item","","food,local",""`;
    });

    // Write the fixed content back
    await writeFile(filePath, fixedLines.join("\n") + "\n", "utf-8");
    console.log(`✓ Fixed ${filename}`);
  } catch (error) {
    console.error(`Error fixing ${filename}:`, error);
  }
}

async function main() {
  console.log("Fixing problematic CSV files...");

  for (const file of problematicFiles) {
    await fixCsvFile(file);
  }

  console.log("\nDone! You can now try importing the data again.");
}

main().catch(console.error);
