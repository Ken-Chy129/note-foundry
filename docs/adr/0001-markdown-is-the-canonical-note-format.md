# Markdown is the canonical note format

Learning Note content is stored canonically as Markdown rather than editor-specific JSON or HTML. A rich-text or live-preview editor may provide the editing experience, but it must read and write the same Markdown representation; this preserves portability, supports technical content, simplifies migration and export, and gives agents a stable text format to inspect and modify.

## Consequences

Editor features must have a lossless Markdown representation. Images and other binary material are attachments referenced by the note, while headings, code blocks, tables, formulas, diagrams, links, and textual citations remain in Markdown.
