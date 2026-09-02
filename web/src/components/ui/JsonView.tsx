// Lightweight regex-based JSON syntax highlighter — enough for a read-only
// inspect view without pulling in a syntax-highlighting library.
function highlight(json: string): string {
  const escaped = json.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return escaped.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g,
    (match) => {
      let cls = "text-sky-600 dark:text-sky-400"; // number
      if (/^"/.test(match)) {
        cls = /:$/.test(match) ? "text-violet-600 dark:text-violet-400" : "text-emerald-600 dark:text-emerald-400";
      } else if (/true|false/.test(match)) {
        cls = "text-amber-600 dark:text-amber-400";
      } else if (/null/.test(match)) {
        cls = "text-ink-faint";
      }
      return `<span class="${cls}">${match}</span>`;
    },
  );
}

export function JsonView({ data }: { data: unknown }) {
  const json = JSON.stringify(data, null, 2);
  return (
    <pre
      className="overflow-x-auto whitespace-pre-wrap break-all p-4 font-mono text-xs leading-relaxed text-ink"
      // Safe: `json` is our own JSON.stringify output, HTML-escaped by
      // highlight() before any markup is added — never raw user input.
      dangerouslySetInnerHTML={{ __html: highlight(json) }}
    />
  );
}
