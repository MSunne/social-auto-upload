const fencedCodePattern = /^ {0,3}(```+|~~~+)/;
const headingPattern = /^#{1,6}\s/;
const unorderedListPattern = /^[-+*]\s/;
const orderedListPattern = /^\d+\.\s/;
const blockquotePattern = /^>\s?/;
const thematicBreakPattern = /^ {0,3}([-*_])(?:\s*\1){2,}\s*$/;

function isBlankLine(line: string) {
  return line.trim().length === 0;
}

function isBlockStarter(line: string) {
  const trimmed = line.trim();
  return (
    headingPattern.test(trimmed) ||
    unorderedListPattern.test(trimmed) ||
    orderedListPattern.test(trimmed) ||
    blockquotePattern.test(trimmed) ||
    thematicBreakPattern.test(trimmed)
  );
}

function isTableDividerLine(line: string) {
  const trimmed = line.trim();
  if (!trimmed.includes("-")) {
    return false;
  }
  return /^\|?(?:\s*:?-{3,}:?\s*\|)+(?:\s*:?-{3,}:?\s*)?$/.test(trimmed);
}

function isTableRowLine(line: string) {
  const trimmed = line.trim();
  if (!trimmed.includes("|")) {
    return false;
  }
  if (isTableDividerLine(trimmed)) {
    return true;
  }
  return trimmed.startsWith("|") || trimmed.endsWith("|");
}

function isPotentialTableStart(currentLine: string, nextLine: string | undefined) {
  if (!nextLine) {
    return false;
  }
  return isTableRowLine(currentLine) && isTableDividerLine(nextLine);
}

function lastNonBlankLine(lines: string[]) {
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    if (!isBlankLine(lines[index])) {
      return lines[index];
    }
  }
  return "";
}

function shouldInsertBlankLineBefore(line: string, previousLines: string[], nextLine: string | undefined) {
  if (previousLines.length === 0) {
    return false;
  }
  const previousLine = lastNonBlankLine(previousLines);
  if (!previousLine || isBlankLine(previousLine)) {
    return false;
  }
  if (isPotentialTableStart(line, nextLine)) {
    return !isTableRowLine(previousLine);
  }
  if (!isBlockStarter(line)) {
    return false;
  }
  if (unorderedListPattern.test(line.trim()) || orderedListPattern.test(line.trim())) {
    return !(unorderedListPattern.test(previousLine.trim()) || orderedListPattern.test(previousLine.trim()));
  }
  return true;
}

function shouldInsertBlankLineAfterTable(currentLine: string, nextLine: string | undefined) {
  if (!nextLine || isBlankLine(nextLine)) {
    return false;
  }
  return isTableRowLine(currentLine) && !isTableRowLine(nextLine);
}

// normalizeChatMarkdown keeps chat markdown readable without rewriting fenced code or tables.
export function normalizeChatMarkdown(content: string) {
  const normalized = content.replace(/\r\n?/g, "\n");
  if (!normalized.trim()) {
    return normalized;
  }

  const inputLines = normalized.split("\n");
  const outputLines: string[] = [];
  let inFence = false;
  let fenceMarker = "";

  for (let index = 0; index < inputLines.length; index += 1) {
    const line = inputLines[index];
    const trimmed = line.trim();
    const nextLine = inputLines[index + 1];
    const fenceMatch = trimmed.match(fencedCodePattern);

    if (fenceMatch) {
      const marker = fenceMatch[1];
      if (!inFence && shouldInsertBlankLineBefore(line, outputLines, nextLine)) {
        outputLines.push("");
      }
      outputLines.push(line);
      if (!inFence) {
        inFence = true;
        fenceMarker = marker[0];
      } else if (marker[0] === fenceMarker) {
        inFence = false;
        fenceMarker = "";
      }
      continue;
    }

    if (inFence) {
      outputLines.push(line);
      continue;
    }

    if (shouldInsertBlankLineBefore(line, outputLines, nextLine)) {
      outputLines.push("");
    }

    outputLines.push(line);

    if (shouldInsertBlankLineAfterTable(line, nextLine)) {
      outputLines.push("");
    }
  }

  return outputLines.join("\n");
}
